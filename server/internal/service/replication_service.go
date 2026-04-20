package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
)

// ReplicationService 实现备份复制（3-2-1 规则核心）。
// 语义：把源备份对象从 source storage target 镜像到 dest target，保持 StoragePath。
//
// 触发路径：
//  1. 自动：BackupExecutionService 备份成功后调用 TriggerAutoReplication
//  2. 手动：前端通过 BackupRecord 详情页触发 Start
//
// 执行模型：异步 + 节点无关（复制在 Master 本地 download → upload）。
// 跨节点 local_disk 场景不支持（与 Download/Delete 保护一致）。
type ReplicationService struct {
	replications    repository.ReplicationRecordRepository
	records         repository.BackupRecordRepository
	targets         repository.StorageTargetRepository
	nodeRepo        repository.NodeRepository
	storageRegistry *storage.Registry
	cipher          *codec.ConfigCipher
	eventDispatcher EventDispatcher
	tempDir         string
	semaphore       chan struct{}
	async           func(func())
	now             func() time.Time
}

func NewReplicationService(
	replications repository.ReplicationRecordRepository,
	records repository.BackupRecordRepository,
	targets repository.StorageTargetRepository,
	nodeRepo repository.NodeRepository,
	storageRegistry *storage.Registry,
	cipher *codec.ConfigCipher,
	tempDir string,
	maxConcurrent int,
) *ReplicationService {
	if tempDir == "" {
		tempDir = "/tmp/backupx-replicate"
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}
	return &ReplicationService{
		replications:    replications,
		records:         records,
		targets:         targets,
		nodeRepo:        nodeRepo,
		storageRegistry: storageRegistry,
		cipher:          cipher,
		tempDir:         tempDir,
		semaphore:       make(chan struct{}, maxConcurrent),
		async:           func(job func()) { go job() },
		now:             func() time.Time { return time.Now().UTC() },
	}
}

func (s *ReplicationService) SetEventDispatcher(dispatcher EventDispatcher) {
	s.eventDispatcher = dispatcher
}

// ReplicationRecordSummary 列表项。
type ReplicationRecordSummary struct {
	ID              uint       `json:"id"`
	BackupRecordID  uint       `json:"backupRecordId"`
	TaskID          uint       `json:"taskId"`
	SourceTargetID  uint       `json:"sourceTargetId"`
	SourceTargetName string    `json:"sourceTargetName"`
	DestTargetID    uint       `json:"destTargetId"`
	DestTargetName  string     `json:"destTargetName"`
	Status          string     `json:"status"`
	StoragePath     string     `json:"storagePath"`
	FileSize        int64      `json:"fileSize"`
	Checksum        string     `json:"checksum"`
	ErrorMessage    string     `json:"errorMessage"`
	DurationSeconds int        `json:"durationSeconds"`
	TriggeredBy     string     `json:"triggeredBy"`
	StartedAt       time.Time  `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
}

type ReplicationRecordListInput struct {
	TaskID         *uint
	BackupRecordID *uint
	DestTargetID   *uint
	Status         string
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}

// TriggerAutoReplication 备份成功钩子：根据 task.ReplicationTargetIDs 自动派发复制。
// best-effort：单个目标失败不影响其他。
func (s *ReplicationService) TriggerAutoReplication(ctx context.Context, task *model.BackupTask, record *model.BackupRecord) {
	if task == nil || record == nil {
		return
	}
	destIDs := parseUintCSV(task.ReplicationTargetIDs)
	if len(destIDs) == 0 {
		return
	}
	// 跨节点 local_disk 场景保护：Master 无法访问远程节点本地文件
	if err := s.validateClusterAccessible(ctx, record); err != nil {
		return
	}
	for _, destID := range destIDs {
		if destID == record.StorageTargetID {
			continue // 源与目标相同，跳过
		}
		_, _ = s.Start(ctx, record.ID, destID, "system")
	}
}

// Start 开始一次复制。同步创建 ReplicationRecord → 异步执行。
func (s *ReplicationService) Start(ctx context.Context, backupRecordID, destTargetID uint, triggeredBy string) (*ReplicationRecordSummary, error) {
	record, err := s.records.FindByID(ctx, backupRecordID)
	if err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_GET_FAILED", "无法获取备份记录", err)
	}
	if record == nil {
		return nil, apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "备份记录不存在", nil)
	}
	if record.Status != model.BackupRecordStatusSuccess {
		return nil, apperror.BadRequest("REPLICATION_SOURCE_INVALID", "只能复制成功的备份记录", nil)
	}
	if destTargetID == 0 || destTargetID == record.StorageTargetID {
		return nil, apperror.BadRequest("REPLICATION_DEST_INVALID", "目标存储无效或与源相同", nil)
	}
	if err := s.validateClusterAccessible(ctx, record); err != nil {
		return nil, err
	}
	dest, err := s.targets.FindByID(ctx, destTargetID)
	if err != nil || dest == nil {
		return nil, apperror.BadRequest("REPLICATION_DEST_INVALID", "目标存储不存在", err)
	}
	if !dest.Enabled {
		return nil, apperror.BadRequest("REPLICATION_DEST_DISABLED", "目标存储已禁用", nil)
	}
	startedAt := s.now()
	rep := &model.ReplicationRecord{
		BackupRecordID: backupRecordID,
		TaskID:         record.TaskID,
		SourceTargetID: record.StorageTargetID,
		DestTargetID:   destTargetID,
		Status:         model.ReplicationStatusRunning,
		StoragePath:    record.StoragePath,
		TriggeredBy:    strings.TrimSpace(triggeredBy),
		StartedAt:      startedAt,
	}
	if err := s.replications.Create(ctx, rep); err != nil {
		return nil, apperror.Internal("REPLICATION_CREATE_FAILED", "无法创建复制记录", err)
	}
	s.async(func() {
		s.executeReplication(context.Background(), rep.ID)
	})
	summary := s.toSummary(rep, "", dest.Name)
	return &summary, nil
}

// executeReplication 实际执行：下载源对象到本地临时文件 → 上传到目标存储。
func (s *ReplicationService) executeReplication(ctx context.Context, repID uint) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	rep, err := s.replications.FindByID(ctx, repID)
	if err != nil || rep == nil {
		return
	}
	status := model.ReplicationStatusFailed
	errMessage := ""
	fileSize := int64(0)

	defer func() {
		completedAt := s.now()
		rep.Status = status
		rep.FileSize = fileSize
		rep.ErrorMessage = strings.TrimSpace(errMessage)
		rep.DurationSeconds = int(completedAt.Sub(rep.StartedAt).Seconds())
		rep.CompletedAt = &completedAt
		_ = s.replications.Update(ctx, rep)
		if status == model.ReplicationStatusFailed {
			s.dispatchFailed(ctx, rep, errMessage)
		}
	}()

	sourceProvider, err := s.resolveProvider(ctx, rep.SourceTargetID)
	if err != nil {
		errMessage = err.Error()
		return
	}
	destProvider, err := s.resolveProvider(ctx, rep.DestTargetID)
	if err != nil {
		errMessage = err.Error()
		return
	}
	if err := os.MkdirAll(s.tempDir, 0o755); err != nil {
		errMessage = err.Error()
		return
	}
	tempDir, err := os.MkdirTemp(s.tempDir, "replicate-*")
	if err != nil {
		errMessage = err.Error()
		return
	}
	defer os.RemoveAll(tempDir)

	reader, err := sourceProvider.Download(ctx, rep.StoragePath)
	if err != nil {
		errMessage = fmt.Sprintf("下载源对象失败: %v", err)
		return
	}
	localPath := filepath.Join(tempDir, filepath.Base(rep.StoragePath))
	if err := writeReaderToFile(localPath, reader); err != nil {
		errMessage = fmt.Sprintf("写入临时文件失败: %v", err)
		return
	}
	info, err := os.Stat(localPath)
	if err != nil {
		errMessage = err.Error()
		return
	}
	fileSize = info.Size()
	file, err := os.Open(localPath)
	if err != nil {
		errMessage = err.Error()
		return
	}
	defer file.Close()
	meta := map[string]string{
		"replicationId": strconv.FormatUint(uint64(rep.ID), 10),
		"sourceRecord":  strconv.FormatUint(uint64(rep.BackupRecordID), 10),
	}
	if err := destProvider.Upload(ctx, rep.StoragePath, file, fileSize, meta); err != nil {
		errMessage = fmt.Sprintf("上传到目标存储失败: %v", err)
		return
	}
	rep.Checksum = "" // 可选：调用方可按需复算 SHA-256
	status = model.ReplicationStatusSuccess
}

func (s *ReplicationService) resolveProvider(ctx context.Context, targetID uint) (storage.StorageProvider, error) {
	target, err := s.targets.FindByID(ctx, targetID)
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_GET_FAILED", "无法获取存储目标", err)
	}
	if target == nil {
		return nil, apperror.BadRequest("STORAGE_TARGET_INVALID", "存储目标不存在", nil)
	}
	configMap := map[string]any{}
	if err := s.cipher.DecryptJSON(target.ConfigCiphertext, &configMap); err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_DECRYPT_FAILED", "无法解密存储配置", err)
	}
	return s.storageRegistry.Create(ctx, target.Type, configMap)
}

// validateClusterAccessible 拒绝跨节点 local_disk 源（Master 无法拉取）
func (s *ReplicationService) validateClusterAccessible(ctx context.Context, record *model.BackupRecord) error {
	if record == nil || record.NodeID == 0 || s.nodeRepo == nil {
		return nil
	}
	node, err := s.nodeRepo.FindByID(ctx, record.NodeID)
	if err != nil || node == nil || node.IsLocal {
		return nil
	}
	target, err := s.targets.FindByID(ctx, record.StorageTargetID)
	if err != nil || target == nil {
		return nil
	}
	if strings.EqualFold(target.Type, "local_disk") {
		return apperror.BadRequest("REPLICATION_CROSS_NODE_LOCAL_DISK",
			fmt.Sprintf("备份位于节点 %s 的本地磁盘（local_disk），Master 无法跨节点复制。请改用云存储作为主备份。", node.Name),
			nil)
	}
	return nil
}

func (s *ReplicationService) dispatchFailed(ctx context.Context, rep *model.ReplicationRecord, message string) {
	if s.eventDispatcher == nil || rep == nil {
		return
	}
	title := "BackupX 备份复制失败"
	body := fmt.Sprintf("备份记录：#%d\n源 → 目标：#%d → #%d\n错误：%s", rep.BackupRecordID, rep.SourceTargetID, rep.DestTargetID, message)
	fields := map[string]any{
		"replicationId":   rep.ID,
		"backupRecordId":  rep.BackupRecordID,
		"taskId":          rep.TaskID,
		"sourceTargetId":  rep.SourceTargetID,
		"destTargetId":    rep.DestTargetID,
		"error":           message,
	}
	_ = s.eventDispatcher.DispatchEvent(ctx, model.NotificationEventReplicationFailed, title, body, fields)
}

// List / Get / toSummary
func (s *ReplicationService) List(ctx context.Context, input ReplicationRecordListInput) ([]ReplicationRecordSummary, error) {
	items, err := s.replications.List(ctx, repository.ReplicationRecordListOptions{
		TaskID: input.TaskID, BackupRecordID: input.BackupRecordID, DestTargetID: input.DestTargetID,
		Status: strings.TrimSpace(input.Status), DateFrom: input.DateFrom, DateTo: input.DateTo,
		Limit: input.Limit, Offset: input.Offset,
	})
	if err != nil {
		return nil, apperror.Internal("REPLICATION_LIST_FAILED", "无法获取复制记录", err)
	}
	result := make([]ReplicationRecordSummary, 0, len(items))
	for i := range items {
		item := items[i]
		result = append(result, s.toSummary(&item, item.SourceTarget.Name, item.DestTarget.Name))
	}
	return result, nil
}

func (s *ReplicationService) Get(ctx context.Context, id uint) (*ReplicationRecordSummary, error) {
	item, err := s.replications.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("REPLICATION_GET_FAILED", "无法获取复制记录", err)
	}
	if item == nil {
		return nil, apperror.New(404, "REPLICATION_NOT_FOUND", "复制记录不存在", nil)
	}
	summary := s.toSummary(item, item.SourceTarget.Name, item.DestTarget.Name)
	return &summary, nil
}

func (s *ReplicationService) toSummary(rep *model.ReplicationRecord, sourceName, destName string) ReplicationRecordSummary {
	return ReplicationRecordSummary{
		ID: rep.ID, BackupRecordID: rep.BackupRecordID, TaskID: rep.TaskID,
		SourceTargetID: rep.SourceTargetID, SourceTargetName: sourceName,
		DestTargetID: rep.DestTargetID, DestTargetName: destName,
		Status: rep.Status, StoragePath: rep.StoragePath, FileSize: rep.FileSize,
		Checksum: rep.Checksum, ErrorMessage: rep.ErrorMessage, DurationSeconds: rep.DurationSeconds,
		TriggeredBy: rep.TriggeredBy, StartedAt: rep.StartedAt, CompletedAt: rep.CompletedAt,
	}
}

// parseUintCSV 解析逗号分隔的 uint 列表，跳过非法项。
func parseUintCSV(value string) []uint {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]uint, 0, len(parts))
	seen := map[uint]bool{}
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		parsed, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil {
			continue
		}
		id := uint(parsed)
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
