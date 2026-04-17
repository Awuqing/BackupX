package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/backup"
	backupretention "backupx/server/internal/backup/retention"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
	"backupx/server/internal/storage/rclone"
	"backupx/server/pkg/compress"
	backupcrypto "backupx/server/pkg/crypto"
)

type BackupExecutionNotification struct {
	Task   *model.BackupTask
	Record *model.BackupRecord
	Error  error
}

type BackupResultNotifier interface {
	NotifyBackupResult(context.Context, BackupExecutionNotification) error
}

type noopBackupNotifier struct{}

func (noopBackupNotifier) NotifyBackupResult(context.Context, BackupExecutionNotification) error {
	return nil
}

type StorageUploadResultItem struct {
	StorageTargetID   uint   `json:"storageTargetId"`
	StorageTargetName string `json:"storageTargetName"`
	Status            string `json:"status"`
	StoragePath       string `json:"storagePath,omitempty"`
	FileSize          int64  `json:"fileSize,omitempty"`
	Error             string `json:"error,omitempty"`
}

type DownloadedArtifact struct {
	FileName string
	Reader   io.ReadCloser
}

// collectTargetIDs 获取任务关联的所有存储目标 ID
func collectTargetIDs(task *model.BackupTask) []uint {
	if len(task.StorageTargets) > 0 {
		ids := make([]uint, len(task.StorageTargets))
		for i, t := range task.StorageTargets {
			ids[i] = t.ID
		}
		return ids
	}
	if task.StorageTargetID > 0 {
		return []uint{task.StorageTargetID}
	}
	return nil
}

type BackupExecutionService struct {
	tasks           repository.BackupTaskRepository
	records         repository.BackupRecordRepository
	targets         repository.StorageTargetRepository
	nodeRepo        repository.NodeRepository
	storageRegistry *storage.Registry
	runnerRegistry  *backup.Registry
	logHub          *backup.LogHub
	retention       *backupretention.Service
	cipher          *codec.ConfigCipher
	notifier        BackupResultNotifier
	agentDispatcher AgentDispatcher
	async           func(func())
	now             func() time.Time
	tempDir         string
	semaphore       chan struct{}
	retries         int    // rclone 底层重试次数
	bandwidthLimit  string // rclone 带宽限制
}

// AgentDispatcher 抽象把任务下发给 Agent 的能力，由 AgentService 实现。
// 用接口避免 execution service ↔ agent service 的循环依赖风险。
type AgentDispatcher interface {
	EnqueueCommand(ctx context.Context, nodeID uint, cmdType string, payload any) (uint, error)
}

// SetClusterDependencies 注入集群相关的依赖，使备份执行时可把任务路由到远程节点。
func (s *BackupExecutionService) SetClusterDependencies(nodeRepo repository.NodeRepository, dispatcher AgentDispatcher) {
	s.nodeRepo = nodeRepo
	s.agentDispatcher = dispatcher
}

func NewBackupExecutionService(
	tasks repository.BackupTaskRepository,
	records repository.BackupRecordRepository,
	targets repository.StorageTargetRepository,
	storageRegistry *storage.Registry,
	runnerRegistry *backup.Registry,
	logHub *backup.LogHub,
	retention *backupretention.Service,
	cipher *codec.ConfigCipher,
	notifier BackupResultNotifier,
	tempDir string,
	maxConcurrent int,
	retries int,
	bandwidthLimit string,
) *BackupExecutionService {
	if notifier == nil {
		notifier = noopBackupNotifier{}
	}
	if tempDir == "" {
		tempDir = "/tmp/backupx"
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}
	return &BackupExecutionService{
		tasks:           tasks,
		records:         records,
		targets:         targets,
		storageRegistry: storageRegistry,
		runnerRegistry:  runnerRegistry,
		logHub:          logHub,
		retention:       retention,
		cipher:          cipher,
		notifier:        notifier,
		async: func(job func()) {
			go job()
		},
		now:            func() time.Time { return time.Now().UTC() },
		tempDir:        tempDir,
		semaphore:      make(chan struct{}, maxConcurrent),
		retries:        retries,
		bandwidthLimit: bandwidthLimit,
	}
}

func (s *BackupExecutionService) RunTaskByID(ctx context.Context, id uint) (*BackupRecordDetail, error) {
	return s.startTask(ctx, id, true)
}

func (s *BackupExecutionService) RunTaskByIDSync(ctx context.Context, id uint) (*BackupRecordDetail, error) {
	return s.startTask(ctx, id, false)
}

func (s *BackupExecutionService) DownloadRecord(ctx context.Context, recordID uint) (*DownloadedArtifact, error) {
	record, provider, err := s.loadRecordProvider(ctx, recordID)
	if err != nil {
		return nil, err
	}
	reader, err := provider.Download(ctx, record.StoragePath)
	if err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_DOWNLOAD_FAILED", "无法下载备份文件", err)
	}
	fileName := record.FileName
	if strings.TrimSpace(fileName) == "" {
		fileName = filepath.Base(record.StoragePath)
	}
	return &DownloadedArtifact{FileName: fileName, Reader: reader}, nil
}

func (s *BackupExecutionService) RestoreRecord(ctx context.Context, recordID uint) error {
	record, provider, err := s.loadRecordProvider(ctx, recordID)
	if err != nil {
		return err
	}
	task, err := s.tasks.FindByID(ctx, record.TaskID)
	if err != nil {
		return apperror.Internal("BACKUP_TASK_GET_FAILED", "无法获取关联备份任务", err)
	}
	if task == nil {
		return apperror.New(404, "BACKUP_TASK_NOT_FOUND", "关联的备份任务不存在，无法执行恢复", fmt.Errorf("backup task %d not found", record.TaskID))
	}
	tempDir, err := os.MkdirTemp("", "backupx-restore-*")
	if err != nil {
		return apperror.Internal("BACKUP_RECORD_RESTORE_FAILED", "无法创建恢复目录", err)
	}
	defer os.RemoveAll(tempDir)
	artifactPath := filepath.Join(tempDir, filepath.Base(record.FileName))
	if strings.TrimSpace(filepath.Base(record.FileName)) == "" {
		artifactPath = filepath.Join(tempDir, filepath.Base(record.StoragePath))
	}
	reader, err := provider.Download(ctx, record.StoragePath)
	if err != nil {
		return apperror.Internal("BACKUP_RECORD_RESTORE_FAILED", "无法下载备份文件", err)
	}
	if err := writeReaderToFile(artifactPath, reader); err != nil {
		return apperror.Internal("BACKUP_RECORD_RESTORE_FAILED", "无法写入恢复文件", err)
	}
	preparedPath, err := s.prepareArtifactForRestore(artifactPath)
	if err != nil {
		return apperror.Internal("BACKUP_RECORD_RESTORE_FAILED", "无法准备恢复文件", err)
	}
	spec, err := s.buildTaskSpec(task, record.StartedAt)
	if err != nil {
		return err
	}
	runner, err := s.runnerRegistry.Runner(spec.Type)
	if err != nil {
		return apperror.BadRequest("BACKUP_TASK_INVALID", "不支持的备份任务类型", err)
	}
	if err := runner.Restore(ctx, spec, preparedPath, backup.NopLogWriter{}); err != nil {
		return apperror.Internal("BACKUP_RECORD_RESTORE_FAILED", "恢复备份失败", err)
	}
	return nil
}

func (s *BackupExecutionService) DeleteRecord(ctx context.Context, recordID uint) error {
	record, provider, err := s.loadRecordProvider(ctx, recordID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(record.StoragePath) != "" {
		if err := provider.Delete(ctx, record.StoragePath); err != nil {
			return apperror.Internal("BACKUP_RECORD_DELETE_FAILED", "无法删除备份文件", err)
		}
	}
	if err := s.records.Delete(ctx, recordID); err != nil {
		return apperror.Internal("BACKUP_RECORD_DELETE_FAILED", "无法删除备份记录", err)
	}
	return nil
}

func (s *BackupExecutionService) startTask(ctx context.Context, id uint, async bool) (*BackupRecordDetail, error) {
	task, err := s.tasks.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_GET_FAILED", "无法获取备份任务详情", err)
	}
	if task == nil {
		return nil, apperror.New(404, "BACKUP_TASK_NOT_FOUND", "备份任务不存在", fmt.Errorf("backup task %d not found", id))
	}
	startedAt := s.now()
	// 取第一个存储目标 ID 做兼容
	primaryTargetID := task.StorageTargetID
	if tids := collectTargetIDs(task); len(tids) > 0 {
		primaryTargetID = tids[0]
	}
	record := &model.BackupRecord{TaskID: task.ID, StorageTargetID: primaryTargetID, Status: "running", StartedAt: startedAt}
	if err := s.records.Create(ctx, record); err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_CREATE_FAILED", "无法创建备份记录", err)
	}
	task.LastRunAt = &startedAt
	task.LastStatus = "running"
	if err := s.tasks.Update(ctx, task); err != nil {
		return nil, apperror.Internal("BACKUP_TASK_UPDATE_FAILED", "无法更新任务状态", err)
	}
	// 多节点路由：task.NodeID 指向远程节点时，把执行任务入队给 Agent；
	// NodeID=0 或本机节点时由 Master 直接执行。
	if s.isRemoteNode(ctx, task.NodeID) {
		if _, enqueueErr := s.agentDispatcher.EnqueueCommand(ctx, task.NodeID, model.AgentCommandTypeRunTask, map[string]any{
			"taskId":   task.ID,
			"recordId": record.ID,
		}); enqueueErr != nil {
			// 入队失败 → 在记录中标记失败，继续返回详情
			_ = s.finalizeRecord(ctx, task, record.ID, startedAt, model.BackupRecordStatusFailed,
				"无法下发任务到远程节点: "+enqueueErr.Error(), "", "", 0, "", "")
			return nil, apperror.Internal("AGENT_COMMAND_ENQUEUE_FAILED", "无法下发任务到远程节点", enqueueErr)
		}
		return s.getRecordDetail(ctx, record.ID)
	}
	run := func() {
		s.executeTask(context.Background(), task, record.ID, startedAt)
	}
	if async {
		s.async(run)
	} else {
		run()
	}
	return s.getRecordDetail(ctx, record.ID)
}

// isRemoteNode 判断 NodeID 是否指向一个有效的远程（非本机）节点。
// 当未注入集群依赖、nodeID 为 0、或节点为本机时，均返回 false（走本地执行）。
func (s *BackupExecutionService) isRemoteNode(ctx context.Context, nodeID uint) bool {
	if s.nodeRepo == nil || s.agentDispatcher == nil || nodeID == 0 {
		return false
	}
	node, err := s.nodeRepo.FindByID(ctx, nodeID)
	if err != nil || node == nil {
		return false
	}
	return !node.IsLocal
}

func (s *BackupExecutionService) executeTask(ctx context.Context, task *model.BackupTask, recordID uint, startedAt time.Time) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	logger := backup.NewExecutionLogger(recordID, s.logHub)
	status := "failed"
	errMessage := ""
	var fileName string
	var fileSize int64
	var checksum string
	var storagePath string
	var uploadResults []StorageUploadResultItem
	completeRecord := func() {
		if finalizeErr := s.finalizeRecord(ctx, task, recordID, startedAt, status, errMessage, logger.String(), fileName, fileSize, checksum, storagePath); finalizeErr != nil {
			logger.Errorf("写回备份记录失败：%v", finalizeErr)
		}
		// 写入多目标上传结果
		if len(uploadResults) > 0 {
			if resultsJSON, marshalErr := json.Marshal(uploadResults); marshalErr == nil {
				if record, findErr := s.records.FindByID(ctx, recordID); findErr == nil && record != nil {
					record.StorageUploadResults = string(resultsJSON)
					_ = s.records.Update(ctx, record)
				}
			}
		}
		if err := s.notifier.NotifyBackupResult(ctx, BackupExecutionNotification{Task: task, Record: &model.BackupRecord{ID: recordID, TaskID: task.ID, Status: status, FileName: fileName, FileSize: fileSize, StoragePath: storagePath, ErrorMessage: errMessage, StartedAt: startedAt}, Error: buildOptionalError(errMessage)}); err != nil {
			logger.Warnf("发送备份通知失败：%v", err)
		}
		s.logHub.Complete(recordID, status)
	}
	defer completeRecord()

	spec, err := s.buildTaskSpec(task, startedAt)
	if err != nil {
		errMessage = err.Error()
		logger.Errorf("构建任务运行时配置失败：%v", err)
		return
	}
	runner, err := s.runnerRegistry.Runner(spec.Type)
	if err != nil {
		errMessage = err.Error()
		logger.Errorf("获取备份执行器失败：%v", err)
		return
	}
	result, err := runner.Run(ctx, spec, logger)
	if err != nil {
		errMessage = err.Error()
		logger.Errorf("执行备份失败：%v", err)
		return
	}
	defer os.RemoveAll(result.TempDir)
	finalPath := result.ArtifactPath
	if strings.EqualFold(task.Compression, "gzip") && !strings.HasSuffix(strings.ToLower(finalPath), ".gz") {
		logger.Infof("开始压缩备份文件")
		compressedPath, compressErr := compress.GzipFile(finalPath)
		if compressErr != nil {
			errMessage = compressErr.Error()
			logger.Errorf("压缩备份文件失败：%v", compressErr)
			return
		}
		finalPath = compressedPath
	}
	if task.Encrypt {
		logger.Infof("开始加密备份文件")
		encryptedPath, encryptErr := backupcrypto.EncryptFile(s.cipher.Key(), finalPath)
		if encryptErr != nil {
			errMessage = encryptErr.Error()
			logger.Errorf("加密备份文件失败：%v", encryptErr)
			return
		}
		finalPath = encryptedPath
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		errMessage = err.Error()
		logger.Errorf("获取备份文件信息失败：%v", err)
		return
	}
	fileSize = info.Size()
	fileName = filepath.Base(finalPath)
	storagePath = backup.BuildStorageKey(task.Type, startedAt, fileName)

	// 收集所有存储目标
	targetIDs := collectTargetIDs(task)
	if len(targetIDs) == 0 {
		errMessage = "没有关联的存储目标"
		logger.Errorf("没有关联的存储目标")
		return
	}

	// 并行上传到所有目标
	uploadResults = make([]StorageUploadResultItem, len(targetIDs))
	var checksumOnce sync.Once
	var wg sync.WaitGroup
	for i, tid := range targetIDs {
		wg.Add(1)
		go func(index int, targetID uint) {
			defer wg.Done()
			target, findErr := s.targets.FindByID(ctx, targetID)
			targetName := fmt.Sprintf("target-%d", targetID)
			if findErr == nil && target != nil {
				targetName = target.Name
			}
			provider, resolveErr := s.resolveProvider(ctx, targetID)
			if resolveErr != nil {
				uploadResults[index] = StorageUploadResultItem{StorageTargetID: targetID, StorageTargetName: targetName, Status: "failed", Error: resolveErr.Error()}
				logger.Warnf("存储目标 %s 创建客户端失败：%v", targetName, resolveErr)
				return
			}
			logger.Infof("开始上传备份到存储目标：%s", targetName)
			// 上传级重试：最多 3 次，指数退避（10s, 30s, 90s）
			maxAttempts := 3
			var lastUploadErr error
			var hr *hashingReader
			for attempt := 1; attempt <= maxAttempts; attempt++ {
				if attempt > 1 {
					backoff := time.Duration(attempt*attempt) * 10 * time.Second
					logger.Warnf("存储目标 %s 第 %d 次重试（等待 %v）：%v", targetName, attempt, backoff, lastUploadErr)
					time.Sleep(backoff)
				}
				artifact, openErr := os.Open(finalPath)
				if openErr != nil {
					uploadResults[index] = StorageUploadResultItem{StorageTargetID: targetID, StorageTargetName: targetName, Status: "failed", Error: openErr.Error()}
					logger.Warnf("存储目标 %s 打开备份文件失败：%v", targetName, openErr)
					return
				}
				hr = newHashingReader(artifact)
				pr := newProgressReader(hr, fileSize, func(bytesRead int64, speedBps float64) {
					percent := float64(0)
					if fileSize > 0 {
						percent = float64(bytesRead) / float64(fileSize) * 100
					}
					s.logHub.AppendProgress(recordID, backup.ProgressInfo{
						BytesSent:  bytesRead,
						TotalBytes: fileSize,
						Percent:    percent,
						SpeedBps:   speedBps,
						TargetName: targetName,
					})
				})
				lastUploadErr = provider.Upload(ctx, storagePath, pr, fileSize, map[string]string{"taskId": fmt.Sprintf("%d", task.ID), "recordId": fmt.Sprintf("%d", recordID)})
				artifact.Close()
				if lastUploadErr == nil {
					break
				}
			}
			if lastUploadErr != nil {
				uploadResults[index] = StorageUploadResultItem{StorageTargetID: targetID, StorageTargetName: targetName, Status: "failed", Error: lastUploadErr.Error()}
				logger.Warnf("存储目标 %s 上传失败（已重试 %d 次）：%v", targetName, maxAttempts, lastUploadErr)
				return
			}
			// 完整性校验：对比实际传输字节数
			if hr.n != fileSize {
				uploadResults[index] = StorageUploadResultItem{StorageTargetID: targetID, StorageTargetName: targetName, Status: "failed", Error: fmt.Sprintf("完整性校验失败: 预期 %d bytes, 实际传输 %d bytes", fileSize, hr.n)}
				logger.Errorf("存储目标 %s 完整性校验失败：预期 %d bytes, 实际传输 %d bytes", targetName, fileSize, hr.n)
				_ = provider.Delete(ctx, storagePath)
				return
			}
			// 取第一个成功目标的哈希写入 record（所有目标读同一文件，哈希一定相同）
			targetChecksum := hr.Sum()
			checksumOnce.Do(func() { checksum = targetChecksum })
			uploadResults[index] = StorageUploadResultItem{StorageTargetID: targetID, StorageTargetName: targetName, Status: "success", StoragePath: storagePath, FileSize: fileSize}
			logger.Infof("存储目标 %s 上传成功 (%d bytes, SHA-256=%s)", targetName, fileSize, targetChecksum)
			// 每个成功目标独立执行保留策略
			if s.retention != nil {
				cleanupResult, cleanupErr := s.retention.Cleanup(ctx, task, provider)
				if cleanupErr != nil {
					logger.Warnf("存储目标 %s 执行保留策略失败：%v", targetName, cleanupErr)
				} else {
					for _, warning := range cleanupResult.Warnings {
						logger.Warnf("存储目标 %s 保留策略警告：%s", targetName, warning)
					}
				}
			}
		}(i, tid)
	}
	wg.Wait()

	// 汇总结果：任意一个 success → 整体 success
	anySuccess := false
	var failedMessages []string
	for _, r := range uploadResults {
		if r.Status == "success" {
			anySuccess = true
		} else if r.Error != "" {
			failedMessages = append(failedMessages, fmt.Sprintf("%s: %s", r.StorageTargetName, r.Error))
		}
	}
	if anySuccess {
		status = "success"
		if len(failedMessages) > 0 {
			logger.Warnf("部分存储目标上传失败：%s", strings.Join(failedMessages, "; "))
		}
		logger.Infof("备份执行完成")
	} else {
		errMessage = strings.Join(failedMessages, "; ")
		logger.Errorf("所有存储目标上传均失败")
	}
}

func (s *BackupExecutionService) finalizeRecord(ctx context.Context, task *model.BackupTask, recordID uint, startedAt time.Time, status string, errorMessage string, logContent string, fileName string, fileSize int64, checksum string, storagePath string) error {
	record, err := s.records.FindByID(ctx, recordID)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("backup record %d not found", recordID)
	}
	completedAt := s.now()
	record.Status = status
	record.FileName = fileName
	record.FileSize = fileSize
	record.Checksum = checksum
	record.StoragePath = storagePath
	record.DurationSeconds = int(completedAt.Sub(startedAt).Seconds())
	record.ErrorMessage = strings.TrimSpace(errorMessage)
	record.LogContent = strings.TrimSpace(logContent)
	record.CompletedAt = &completedAt
	if err := s.records.Update(ctx, record); err != nil {
		return err
	}
	task.LastRunAt = &startedAt
	task.LastStatus = status
	return s.tasks.Update(ctx, task)
}

func (s *BackupExecutionService) resolveProvider(ctx context.Context, targetID uint) (storage.StorageProvider, error) {
	// 注入 rclone 传输配置（重试、带宽限制）
	ctx = rclone.ConfiguredContext(ctx, rclone.TransferConfig{
		LowLevelRetries: s.retries,
		BandwidthLimit:  s.bandwidthLimit,
	})
	target, err := s.targets.FindByID(ctx, targetID)
	if err != nil {
		return nil, apperror.Internal("BACKUP_STORAGE_TARGET_GET_FAILED", "无法获取存储目标详情", err)
	}
	if target == nil {
		return nil, apperror.BadRequest("BACKUP_STORAGE_TARGET_INVALID", "关联的存储目标不存在", nil)
	}
	configMap := map[string]any{}
	if err := s.cipher.DecryptJSON(target.ConfigCiphertext, &configMap); err != nil {
		return nil, apperror.Internal("BACKUP_STORAGE_TARGET_DECRYPT_FAILED", "无法解密存储目标配置", err)
	}
	provider, err := s.storageRegistry.Create(ctx, target.Type, configMap)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

func (s *BackupExecutionService) buildTaskSpec(task *model.BackupTask, startedAt time.Time) (backup.TaskSpec, error) {
	excludePatterns := []string{}
	if strings.TrimSpace(task.ExcludePatterns) != "" {
		if err := json.Unmarshal([]byte(task.ExcludePatterns), &excludePatterns); err != nil {
			return backup.TaskSpec{}, apperror.Internal("BACKUP_TASK_DECODE_FAILED", "无法解析排除规则", err)
		}
	}
	password := ""
	if strings.TrimSpace(task.DBPasswordCiphertext) != "" {
		plain, err := s.cipher.Decrypt(task.DBPasswordCiphertext)
		if err != nil {
			return backup.TaskSpec{}, apperror.Internal("BACKUP_TASK_DECRYPT_FAILED", "无法解密数据库密码", err)
		}
		password = string(plain)
	}
	sourcePaths := []string{}
	if strings.TrimSpace(task.SourcePaths) != "" {
		if err := json.Unmarshal([]byte(task.SourcePaths), &sourcePaths); err != nil {
			return backup.TaskSpec{}, apperror.Internal("BACKUP_TASK_DECODE_FAILED", "无法解析源路径配置", err)
		}
	}
	dbSpec := backup.DatabaseSpec{
		Host:     task.DBHost,
		Port:     task.DBPort,
		User:     task.DBUser,
		Password: password,
		Names:    []string{task.DBName},
		Path:     task.DBPath,
	}
	// 解析 ExtraConfig 填充类型特有字段（目前主要用于 SAP HANA）
	if strings.TrimSpace(task.ExtraConfig) != "" {
		extra := map[string]any{}
		if err := json.Unmarshal([]byte(task.ExtraConfig), &extra); err != nil {
			return backup.TaskSpec{}, apperror.Internal("BACKUP_TASK_DECODE_FAILED", "无法解析扩展配置", err)
		}
		applyHANAExtraConfig(&dbSpec, extra)
	}
	return backup.TaskSpec{
		ID:                task.ID,
		Name:              task.Name,
		Type:              task.Type,
		SourcePath:        task.SourcePath,
		SourcePaths:       sourcePaths,
		ExcludePatterns:   excludePatterns,
		StorageTargetID:   task.StorageTargetID,
		StorageTargetType: "",
		Compression:       task.Compression,
		Encrypt:           task.Encrypt,
		RetentionDays:     task.RetentionDays,
		MaxBackups:        task.MaxBackups,
		StartedAt:         startedAt,
		TempDir:           s.tempDir,
		Database:          dbSpec,
	}, nil
}

// applyHANAExtraConfig 从 ExtraConfig map 中提取 SAP HANA 字段填入 DatabaseSpec。
// 不识别的键被忽略，保持向后兼容。
func applyHANAExtraConfig(spec *backup.DatabaseSpec, extra map[string]any) {
	if v, ok := extra["instanceNumber"].(string); ok {
		spec.InstanceNumber = strings.TrimSpace(v)
	}
	if v, ok := extra["backupLevel"].(string); ok {
		spec.BackupLevel = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := extra["backupType"].(string); ok {
		spec.BackupType = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := extra["backupChannels"].(float64); ok {
		spec.BackupChannels = int(v)
	}
	if v, ok := extra["maxRetries"].(float64); ok {
		spec.MaxRetries = int(v)
	}
}

func (s *BackupExecutionService) loadRecordProvider(ctx context.Context, recordID uint) (*model.BackupRecord, storage.StorageProvider, error) {
	record, err := s.records.FindByID(ctx, recordID)
	if err != nil {
		return nil, nil, apperror.Internal("BACKUP_RECORD_GET_FAILED", "无法获取备份记录详情", err)
	}
	if record == nil {
		return nil, nil, apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "备份记录不存在", fmt.Errorf("backup record %d not found", recordID))
	}
	provider, err := s.resolveProvider(ctx, record.StorageTargetID)
	if err != nil {
		return nil, nil, err
	}
	return record, provider, nil
}

func (s *BackupExecutionService) prepareArtifactForRestore(artifactPath string) (string, error) {
	currentPath := artifactPath
	if strings.HasSuffix(strings.ToLower(currentPath), ".enc") {
		decryptedPath, err := backupcrypto.DecryptFile(s.cipher.Key(), currentPath)
		if err != nil {
			return "", err
		}
		currentPath = decryptedPath
	}
	if strings.HasSuffix(strings.ToLower(currentPath), ".gz") {
		decompressedPath, err := compress.GunzipFile(currentPath)
		if err != nil {
			return "", err
		}
		currentPath = decompressedPath
	}
	return currentPath, nil
}

func (s *BackupExecutionService) getRecordDetail(ctx context.Context, recordID uint) (*BackupRecordDetail, error) {
	record, err := s.records.FindByID(ctx, recordID)
	if err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_GET_FAILED", "无法获取备份记录详情", err)
	}
	if record == nil {
		return nil, apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "备份记录不存在", fmt.Errorf("backup record %d not found", recordID))
	}
	return toBackupRecordDetail(record, s.logHub), nil
}

func writeReaderToFile(targetPath string, reader io.ReadCloser) error {
	defer reader.Close()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, reader)
	return err
}

func buildOptionalError(message string) error {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	return fmt.Errorf("%s", message)
}

func buildStorageProviderFromRepos(ctx context.Context, storageTargetID uint, storageTargets repository.StorageTargetRepository, storageRegistry *storage.Registry, cipher *codec.ConfigCipher) (storage.StorageProvider, *model.StorageTarget, error) {
	target, err := storageTargets.FindByID(ctx, storageTargetID)
	if err != nil {
		return nil, nil, apperror.Internal("BACKUP_STORAGE_TARGET_LOOKUP_FAILED", "无法读取存储目标", err)
	}
	if target == nil {
		return nil, nil, apperror.BadRequest("BACKUP_STORAGE_TARGET_INVALID", "存储目标不存在", nil)
	}
	var configMap map[string]any
	if err := cipher.DecryptJSON(target.ConfigCiphertext, &configMap); err != nil {
		return nil, nil, apperror.Internal("BACKUP_STORAGE_TARGET_DECRYPT_FAILED", "无法解密存储目标配置", err)
	}
	provider, err := storageRegistry.Create(ctx, storage.ParseProviderType(target.Type), configMap)
	if err != nil {
		return nil, nil, err
	}
	return provider, target, nil
}

// hashingReader 在上传过程中同步计算字节数和 SHA-256，零额外 I/O
type hashingReader struct {
	reader io.Reader
	hash   hash.Hash
	n      int64
}

func newHashingReader(reader io.Reader) *hashingReader {
	h := sha256.New()
	return &hashingReader{
		reader: io.TeeReader(reader, h),
		hash:   h,
	}
}

func (r *hashingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.n += int64(n)
	return n, err
}

func (r *hashingReader) Sum() string {
	return hex.EncodeToString(r.hash.Sum(nil))
}
