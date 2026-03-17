package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/backup"
	backupretention "backupx/server/internal/backup/retention"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
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

type DownloadedArtifact struct {
	FileName string
	Reader   io.ReadCloser
}

type BackupExecutionService struct {
	tasks           repository.BackupTaskRepository
	records         repository.BackupRecordRepository
	targets         repository.StorageTargetRepository
	storageRegistry *storage.Registry
	runnerRegistry  *backup.Registry
	logHub          *backup.LogHub
	retention       *backupretention.Service
	cipher          *codec.ConfigCipher
	notifier        BackupResultNotifier
	async           func(func())
	now             func() time.Time
	tempDir         string
	semaphore       chan struct{}
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
		now:       func() time.Time { return time.Now().UTC() },
		tempDir:   tempDir,
		semaphore: make(chan struct{}, maxConcurrent),
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
	record := &model.BackupRecord{TaskID: task.ID, StorageTargetID: task.StorageTargetID, Status: "running", StartedAt: startedAt}
	if err := s.records.Create(ctx, record); err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_CREATE_FAILED", "无法创建备份记录", err)
	}
	task.LastRunAt = &startedAt
	task.LastStatus = "running"
	if err := s.tasks.Update(ctx, task); err != nil {
		return nil, apperror.Internal("BACKUP_TASK_UPDATE_FAILED", "无法更新任务状态", err)
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

func (s *BackupExecutionService) executeTask(ctx context.Context, task *model.BackupTask, recordID uint, startedAt time.Time) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	logger := backup.NewExecutionLogger(recordID, s.logHub)
	status := "failed"
	errMessage := ""
	var fileName string
	var fileSize int64
	var storagePath string
	completeRecord := func() {
		if finalizeErr := s.finalizeRecord(ctx, task, recordID, startedAt, status, errMessage, logger.String(), fileName, fileSize, storagePath); finalizeErr != nil {
			logger.Errorf("写回备份记录失败：%v", finalizeErr)
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
	provider, err := s.resolveProvider(ctx, task.StorageTargetID)
	if err != nil {
		errMessage = err.Error()
		logger.Errorf("创建存储客户端失败：%v", err)
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
	artifact, err := os.Open(finalPath)
	if err != nil {
		errMessage = err.Error()
		logger.Errorf("打开备份文件失败：%v", err)
		return
	}
	defer artifact.Close()
	logger.Infof("开始上传备份到存储目标")
	if err := provider.Upload(ctx, storagePath, artifact, fileSize, map[string]string{"taskId": fmt.Sprintf("%d", task.ID), "recordId": fmt.Sprintf("%d", recordID)}); err != nil {
		errMessage = err.Error()
		logger.Errorf("上传备份文件失败：%v", err)
		return
	}
	if s.retention != nil {
		cleanupResult, cleanupErr := s.retention.Cleanup(ctx, task, provider)
		if cleanupErr != nil {
			logger.Warnf("执行保留策略失败：%v", cleanupErr)
		} else {
			for _, warning := range cleanupResult.Warnings {
				logger.Warnf("保留策略警告：%s", warning)
			}
		}
	}
	status = "success"
	logger.Infof("备份执行完成")
}

func (s *BackupExecutionService) finalizeRecord(ctx context.Context, task *model.BackupTask, recordID uint, startedAt time.Time, status string, errorMessage string, logContent string, fileName string, fileSize int64, storagePath string) error {
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
	return backup.TaskSpec{
		ID:                task.ID,
		Name:              task.Name,
		Type:              task.Type,
		SourcePath:        task.SourcePath,
		ExcludePatterns:   excludePatterns,
		StorageTargetID:   task.StorageTargetID,
		StorageTargetType: "",
		Compression:       task.Compression,
		Encrypt:           task.Encrypt,
		RetentionDays:     task.RetentionDays,
		MaxBackups:        task.MaxBackups,
		StartedAt:         startedAt,
		TempDir:           s.tempDir,
		Database: backup.DatabaseSpec{
			Host:     task.DBHost,
			Port:     task.DBPort,
			User:     task.DBUser,
			Password: password,
			Names:    []string{task.DBName},
			Path:     task.DBPath,
		},
	}, nil
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
