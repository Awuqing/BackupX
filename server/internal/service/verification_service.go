package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/backup"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
	"backupx/server/pkg/compress"
	backupcrypto "backupx/server/pkg/crypto"
)

// VerificationService 管理备份验证（恢复演练）记录生命周期。
//
// 执行模型 v1：仅在 Master 本地执行。
//   - 下载备份对象到临时沙箱（local_disk 跨节点场景因 Master 取不到远程文件而失败；
//     返回明确错误告知用户）
//   - 解密 + 解压
//   - 按任务类型调用 backup.Verify* 家族的 quick 校验
//   - 不触碰任务源数据
//
// Agent 侧执行（远程节点直接验证本地备份）作为未来扩展点。
type VerificationService struct {
	verifications   repository.VerificationRecordRepository
	records         repository.BackupRecordRepository
	tasks           repository.BackupTaskRepository
	targets         repository.StorageTargetRepository
	nodeRepo        repository.NodeRepository
	storageRegistry *storage.Registry
	logHub          *backup.LogHub
	cipher          *codec.ConfigCipher
	notifier        VerificationNotifier
	tempDir         string
	semaphore       chan struct{}
	async           func(func())
	now             func() time.Time
}

// VerificationNotifier 给用户推送验证完成/失败通知。
// 可选注入：未注入时仅写记录。
type VerificationNotifier interface {
	NotifyVerificationResult(ctx context.Context, task *model.BackupTask, record *model.VerificationRecord) error
}

type noopVerificationNotifier struct{}

func (noopVerificationNotifier) NotifyVerificationResult(context.Context, *model.BackupTask, *model.VerificationRecord) error {
	return nil
}

// VerificationEventNotifier 适配 NotificationService 的事件分发，面向 verify_failed 事件。
type VerificationEventNotifier struct {
	dispatcher EventDispatcher
}

// EventDispatcher 抽象事件派发（实现者：NotificationService）。
type EventDispatcher interface {
	DispatchEvent(ctx context.Context, eventType, title, body string, fields map[string]any) error
}

// NewVerificationEventNotifier 构造一个事件分发 adapter。dispatcher 为 nil 时退化为 noop。
func NewVerificationEventNotifier(dispatcher EventDispatcher) VerificationNotifier {
	if dispatcher == nil {
		return noopVerificationNotifier{}
	}
	return &VerificationEventNotifier{dispatcher: dispatcher}
}

func (v *VerificationEventNotifier) NotifyVerificationResult(ctx context.Context, task *model.BackupTask, record *model.VerificationRecord) error {
	if record == nil || record.Status != model.VerificationRecordStatusFailed {
		return nil
	}
	taskName := "未知任务"
	if task != nil {
		taskName = task.Name
	}
	title := "BackupX 备份验证失败"
	body := fmt.Sprintf("任务：%s\n验证记录：#%d\n错误：%s", taskName, record.ID, record.ErrorMessage)
	fields := map[string]any{
		"taskId":        record.TaskID,
		"taskName":      taskName,
		"verifyId":      record.ID,
		"backupRecordId": record.BackupRecordID,
		"error":         record.ErrorMessage,
	}
	return v.dispatcher.DispatchEvent(ctx, model.NotificationEventVerifyFailed, title, body, fields)
}

func NewVerificationService(
	verifications repository.VerificationRecordRepository,
	records repository.BackupRecordRepository,
	tasks repository.BackupTaskRepository,
	targets repository.StorageTargetRepository,
	nodeRepo repository.NodeRepository,
	storageRegistry *storage.Registry,
	logHub *backup.LogHub,
	cipher *codec.ConfigCipher,
	tempDir string,
	maxConcurrent int,
) *VerificationService {
	if tempDir == "" {
		tempDir = "/tmp/backupx-verify"
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}
	return &VerificationService{
		verifications:   verifications,
		records:         records,
		tasks:           tasks,
		targets:         targets,
		nodeRepo:        nodeRepo,
		storageRegistry: storageRegistry,
		logHub:          logHub,
		cipher:          cipher,
		notifier:        noopVerificationNotifier{},
		tempDir:         tempDir,
		semaphore:       make(chan struct{}, maxConcurrent),
		async:           func(job func()) { go job() },
		now:             func() time.Time { return time.Now().UTC() },
	}
}

// SetNotifier 注入通知器。
func (s *VerificationService) SetNotifier(notifier VerificationNotifier) {
	if notifier != nil {
		s.notifier = notifier
	}
}

// VerificationRecordSummary 列表项。
type VerificationRecordSummary struct {
	ID              uint       `json:"id"`
	BackupRecordID  uint       `json:"backupRecordId"`
	TaskID          uint       `json:"taskId"`
	TaskName        string     `json:"taskName"`
	NodeID          uint       `json:"nodeId"`
	Mode            string     `json:"mode"`
	Status          string     `json:"status"`
	Summary         string     `json:"summary"`
	ErrorMessage    string     `json:"errorMessage"`
	DurationSeconds int        `json:"durationSeconds"`
	StartedAt       time.Time  `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	TriggeredBy     string     `json:"triggeredBy"`
	BackupFileName  string     `json:"backupFileName,omitempty"`
}

type VerificationRecordDetail struct {
	VerificationRecordSummary
	LogContent string            `json:"logContent"`
	LogEvents  []backup.LogEvent `json:"logEvents,omitempty"`
}

type VerificationRecordListInput struct {
	TaskID         *uint
	BackupRecordID *uint
	Status         string
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}

// StartByTask 从指定任务的"最新成功备份"触发一次验证。
// 常用于调度器或手动 UI 按钮。
func (s *VerificationService) StartByTask(ctx context.Context, taskID uint, mode, triggeredBy string) (*VerificationRecordDetail, error) {
	records, err := s.records.ListSuccessfulByTask(ctx, taskID)
	if err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_LIST_FAILED", "无法获取备份记录", err)
	}
	if len(records) == 0 {
		return nil, apperror.BadRequest("VERIFY_NO_SOURCE", "该任务尚无成功的备份记录可验证", nil)
	}
	return s.Start(ctx, records[0].ID, mode, triggeredBy)
}

// Start 触发一次验证。创建 VerificationRecord → 异步本地执行。
func (s *VerificationService) Start(ctx context.Context, backupRecordID uint, mode, triggeredBy string) (*VerificationRecordDetail, error) {
	record, err := s.records.FindByID(ctx, backupRecordID)
	if err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_GET_FAILED", "无法获取备份记录", err)
	}
	if record == nil {
		return nil, apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "备份记录不存在", nil)
	}
	if record.Status != model.BackupRecordStatusSuccess {
		return nil, apperror.BadRequest("VERIFY_SOURCE_INVALID", "只能验证状态为成功的备份记录", nil)
	}
	task, err := s.tasks.FindByID(ctx, record.TaskID)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_GET_FAILED", "无法获取关联任务", err)
	}
	if task == nil {
		return nil, apperror.New(404, "BACKUP_TASK_NOT_FOUND", "关联的备份任务不存在", nil)
	}
	// 集群场景保护：跨节点 local_disk 备份 Master 取不到 → 拒绝并提示
	if err := s.validateClusterAccessible(ctx, record); err != nil {
		return nil, err
	}
	if mode == "" {
		mode = model.VerificationModeQuick
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != model.VerificationModeQuick && mode != model.VerificationModeDeep {
		return nil, apperror.BadRequest("VERIFY_MODE_INVALID", "不支持的验证模式", nil)
	}
	startedAt := s.now()
	verification := &model.VerificationRecord{
		BackupRecordID: backupRecordID,
		TaskID:         record.TaskID,
		NodeID:         record.NodeID,
		Mode:           mode,
		Status:         model.VerificationRecordStatusRunning,
		StartedAt:      startedAt,
		TriggeredBy:    strings.TrimSpace(triggeredBy),
	}
	if err := s.verifications.Create(ctx, verification); err != nil {
		return nil, apperror.Internal("VERIFY_RECORD_CREATE_FAILED", "无法创建验证记录", err)
	}
	run := func() {
		s.executeLocally(context.Background(), verification.ID, task, record)
	}
	s.async(run)
	return s.getDetail(ctx, verification.ID)
}

// validateClusterAccessible 复刻 BackupExecutionService 的跨节点 local_disk 保护。
// 避免 Master 端在错误机器下载/校验到假数据。
func (s *VerificationService) validateClusterAccessible(ctx context.Context, record *model.BackupRecord) error {
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
		return apperror.BadRequest("VERIFY_CROSS_NODE_LOCAL_DISK",
			fmt.Sprintf("备份位于节点 %s 的本地磁盘（local_disk），Master 无法跨节点验证。", node.Name),
			nil)
	}
	return nil
}

// executeLocally 异步执行验证：下载 → 解密 → 解压 → 按类型校验。
func (s *VerificationService) executeLocally(ctx context.Context, verID uint, task *model.BackupTask, backupRecord *model.BackupRecord) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	logger := backup.NewExecutionLogger(verID, s.logHub)
	status := model.VerificationRecordStatusFailed
	errMessage := ""
	summary := ""

	defer func() {
		_ = s.finalize(ctx, verID, status, errMessage, summary, logger.String())
		s.logHub.Complete(verID, status)
		// 失败时推送通知（best-effort）
		if status == model.VerificationRecordStatusFailed && s.notifier != nil {
			if record, err := s.verifications.FindByID(ctx, verID); err == nil && record != nil {
				_ = s.notifier.NotifyVerificationResult(ctx, task, record)
			}
		}
	}()

	logger.Infof("开始验证备份记录 #%d（模式：%s）", backupRecord.ID, model.VerificationModeQuick)

	if err := os.MkdirAll(s.tempDir, 0o755); err != nil {
		errMessage = err.Error()
		logger.Errorf("创建验证临时父目录失败：%v", err)
		return
	}
	sandbox, err := os.MkdirTemp(s.tempDir, "verify-*")
	if err != nil {
		errMessage = err.Error()
		logger.Errorf("创建沙箱目录失败：%v", err)
		return
	}
	defer os.RemoveAll(sandbox)

	target, err := s.targets.FindByID(ctx, backupRecord.StorageTargetID)
	if err != nil || target == nil {
		errMessage = "存储目标不可用"
		logger.Errorf("获取存储目标失败：%v", err)
		return
	}
	configMap := map[string]any{}
	if err := s.cipher.DecryptJSON(target.ConfigCiphertext, &configMap); err != nil {
		errMessage = err.Error()
		logger.Errorf("解密存储配置失败：%v", err)
		return
	}
	provider, err := s.storageRegistry.Create(ctx, target.Type, configMap)
	if err != nil {
		errMessage = err.Error()
		logger.Errorf("创建存储客户端失败：%v", err)
		return
	}
	fileName := backupRecord.FileName
	if strings.TrimSpace(fileName) == "" {
		fileName = filepath.Base(backupRecord.StoragePath)
	}
	artifactPath := filepath.Join(sandbox, filepath.Base(fileName))
	logger.Infof("下载备份：%s", backupRecord.StoragePath)
	reader, err := provider.Download(ctx, backupRecord.StoragePath)
	if err != nil {
		errMessage = err.Error()
		logger.Errorf("下载备份失败：%v", err)
		return
	}
	if err := writeReaderToFile(artifactPath, reader); err != nil {
		errMessage = err.Error()
		logger.Errorf("写入沙箱失败：%v", err)
		return
	}
	preparedPath, err := s.prepareArtifact(artifactPath, logger)
	if err != nil {
		errMessage = err.Error()
		logger.Errorf("准备归档失败：%v", err)
		return
	}
	// 按任务类型分派校验
	report, verifyErr := s.verifyByType(task.Type, preparedPath, backupRecord.Checksum, logger)
	if verifyErr != nil {
		errMessage = verifyErr.Error()
		if report != nil && report.Detail != "" {
			summary = report.Detail
		}
		logger.Errorf("验证未通过：%v", verifyErr)
		return
	}
	status = model.VerificationRecordStatusSuccess
	if report != nil {
		summary = report.Detail
	}
	logger.Infof("验证通过：%s", summary)
}

// prepareArtifact 按后缀解密/解压，返回可读路径。
func (s *VerificationService) prepareArtifact(artifactPath string, logger *backup.ExecutionLogger) (string, error) {
	current := artifactPath
	if strings.HasSuffix(strings.ToLower(current), ".enc") {
		logger.Infof("检测到加密后缀，开始解密")
		decrypted, err := backupcrypto.DecryptFile(s.cipher.Key(), current)
		if err != nil {
			return "", err
		}
		current = decrypted
	}
	if strings.HasSuffix(strings.ToLower(current), ".gz") {
		logger.Infof("检测到 gzip，解压")
		decompressed, err := compress.GunzipFile(current)
		if err != nil {
			return "", err
		}
		current = decompressed
	}
	return current, nil
}

// verifyByType 按任务类型分派到对应 Verify* 策略。
func (s *VerificationService) verifyByType(taskType, artifactPath, checksum string, logger *backup.ExecutionLogger) (*backup.VerifyReport, error) {
	switch strings.ToLower(strings.TrimSpace(taskType)) {
	case "file":
		logger.Infof("执行文件归档校验")
		return backup.VerifyTarArchive(artifactPath, "")
	case "sqlite":
		logger.Infof("执行 SQLite 文件头校验")
		return backup.VerifySQLiteFile(artifactPath)
	case "mysql":
		logger.Infof("执行 MySQL dump 校验")
		return backup.VerifyMySQLDump(artifactPath)
	case "postgresql":
		logger.Infof("执行 PostgreSQL dump 校验")
		return backup.VerifyPostgreSQLDump(artifactPath)
	case "saphana":
		logger.Infof("执行 SAP HANA 归档校验")
		return backup.VerifySAPHANAArchive(artifactPath)
	default:
		return nil, fmt.Errorf("unsupported task type for verification: %s", taskType)
	}
}

func (s *VerificationService) finalize(ctx context.Context, verID uint, status, errMessage, summary, logContent string) error {
	record, err := s.verifications.FindByID(ctx, verID)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("verification record %d not found", verID)
	}
	completedAt := s.now()
	record.Status = status
	record.ErrorMessage = strings.TrimSpace(errMessage)
	if strings.TrimSpace(summary) != "" {
		record.Summary = summary
	}
	if strings.TrimSpace(logContent) != "" {
		record.LogContent = strings.TrimSpace(logContent)
	}
	record.DurationSeconds = int(completedAt.Sub(record.StartedAt).Seconds())
	record.CompletedAt = &completedAt
	return s.verifications.Update(ctx, record)
}

func (s *VerificationService) Get(ctx context.Context, id uint) (*VerificationRecordDetail, error) {
	return s.getDetail(ctx, id)
}

func (s *VerificationService) List(ctx context.Context, input VerificationRecordListInput) ([]VerificationRecordSummary, error) {
	items, err := s.verifications.List(ctx, repository.VerificationRecordListOptions{
		TaskID:         input.TaskID,
		BackupRecordID: input.BackupRecordID,
		Status:         strings.TrimSpace(input.Status),
		DateFrom:       input.DateFrom,
		DateTo:         input.DateTo,
		Limit:          input.Limit,
		Offset:         input.Offset,
	})
	if err != nil {
		return nil, apperror.Internal("VERIFY_RECORD_LIST_FAILED", "无法获取验证记录列表", err)
	}
	result := make([]VerificationRecordSummary, 0, len(items))
	for i := range items {
		result = append(result, toVerificationSummary(&items[i]))
	}
	return result, nil
}

// LatestByTask 返回任务的最近一次验证记录（nil 表示未验证过）。
// 用于任务详情页显示"最近验证状态"。
func (s *VerificationService) LatestByTask(ctx context.Context, taskID uint) (*VerificationRecordSummary, error) {
	item, err := s.verifications.FindLatestByTask(ctx, taskID)
	if err != nil {
		return nil, apperror.Internal("VERIFY_RECORD_GET_FAILED", "无法获取最新验证记录", err)
	}
	if item == nil {
		return nil, nil
	}
	summary := toVerificationSummary(item)
	return &summary, nil
}

func (s *VerificationService) SubscribeLogs(ctx context.Context, id uint, buffer int) (<-chan backup.LogEvent, func(), error) {
	record, err := s.verifications.FindByID(ctx, id)
	if err != nil {
		return nil, nil, apperror.Internal("VERIFY_RECORD_GET_FAILED", "无法获取验证记录", err)
	}
	if record == nil {
		return nil, nil, apperror.New(404, "VERIFY_RECORD_NOT_FOUND", "验证记录不存在", nil)
	}
	channel, cancel := s.logHub.Subscribe(id, buffer)
	return channel, cancel, nil
}

func (s *VerificationService) getDetail(ctx context.Context, id uint) (*VerificationRecordDetail, error) {
	record, err := s.verifications.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("VERIFY_RECORD_GET_FAILED", "无法获取验证记录详情", err)
	}
	if record == nil {
		return nil, apperror.New(404, "VERIFY_RECORD_NOT_FOUND", "验证记录不存在", nil)
	}
	detail := &VerificationRecordDetail{
		VerificationRecordSummary: toVerificationSummary(record),
		LogContent:                record.LogContent,
	}
	if record.Status == model.VerificationRecordStatusRunning && s.logHub != nil {
		events := s.logHub.Snapshot(record.ID)
		detail.LogEvents = events
		if len(events) > 0 {
			lines := make([]string, 0, len(events))
			for _, event := range events {
				lines = append(lines, event.Message)
			}
			detail.LogContent = strings.Join(lines, "\n")
		}
	}
	return detail, nil
}

func toVerificationSummary(item *model.VerificationRecord) VerificationRecordSummary {
	summary := VerificationRecordSummary{
		ID:              item.ID,
		BackupRecordID:  item.BackupRecordID,
		TaskID:          item.TaskID,
		TaskName:        item.Task.Name,
		NodeID:          item.NodeID,
		Mode:            item.Mode,
		Status:          item.Status,
		Summary:         item.Summary,
		ErrorMessage:    item.ErrorMessage,
		DurationSeconds: item.DurationSeconds,
		StartedAt:       item.StartedAt,
		CompletedAt:     item.CompletedAt,
		TriggeredBy:     item.TriggeredBy,
	}
	if strings.TrimSpace(item.BackupRecord.FileName) != "" {
		summary.BackupFileName = item.BackupRecord.FileName
	}
	return summary
}
