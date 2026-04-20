package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backupx/server/internal/backup"
	"backupx/server/internal/storage"
	storageRclone "backupx/server/internal/storage/rclone"
	"backupx/server/pkg/compress"
)

// Executor 负责在 Agent 本地执行命令。
type Executor struct {
	client           *MasterClient
	tempDir          string
	backupRegistry   *backup.Registry
	storageRegistry  *storage.Registry
}

// NewExecutor 构造执行器。预先初始化 backup runner 与 storage registry。
func NewExecutor(client *MasterClient, tempDir string) *Executor {
	backupRegistry := backup.NewRegistry(
		backup.NewFileRunner(),
		backup.NewSQLiteRunner(),
		backup.NewMySQLRunner(nil),
		backup.NewPostgreSQLRunner(nil),
		backup.NewSAPHANARunner(nil),
	)
	storageRegistry := storage.NewRegistry(
		storageRclone.NewLocalDiskFactory(),
		storageRclone.NewS3Factory(),
		storageRclone.NewWebDAVFactory(),
		storageRclone.NewGoogleDriveFactory(),
		storageRclone.NewAliyunOSSFactory(),
		storageRclone.NewTencentCOSFactory(),
		storageRclone.NewQiniuKodoFactory(),
		storageRclone.NewFTPFactory(),
		storageRclone.NewRcloneFactory(),
	)
	storageRclone.RegisterAllBackends(storageRegistry)
	return &Executor{
		client:          client,
		tempDir:         tempDir,
		backupRegistry:  backupRegistry,
		storageRegistry: storageRegistry,
	}
}

// ExecuteRunTask 处理 run_task 命令：拉规格 → 执行 runner → 压缩 → 上传 → 上报记录。
//
// 注意：Agent 当前不支持 Encrypt=true（加密密钥不下发到 Agent，避免密钥扩散）。
// 遇到启用加密的任务会向 Master 上报失败并返回错误。
func (e *Executor) ExecuteRunTask(ctx context.Context, taskID, recordID uint) error {
	// 1) 拉取任务规格
	spec, err := e.client.GetTaskSpec(ctx, taskID)
	if err != nil {
		e.reportRecordFailure(ctx, recordID, fmt.Sprintf("拉取任务规格失败: %v", err))
		return err
	}
	if spec.Encrypt {
		msg := "Agent 不支持加密备份（加密密钥仅在 Master 端持有）"
		e.reportRecordFailure(ctx, recordID, msg)
		return fmt.Errorf("%s", msg)
	}
	e.appendLog(ctx, recordID, fmt.Sprintf("[agent] 开始执行任务 %s (type=%s)\n", spec.Name, spec.Type))

	// 2) 构造 backup.TaskSpec 并找对应 runner
	startedAt := time.Now().UTC()
	if err := os.MkdirAll(e.tempDir, 0o755); err != nil {
		e.reportRecordFailure(ctx, recordID, fmt.Sprintf("创建临时目录失败: %v", err))
		return err
	}
	backupSpec := buildBackupTaskSpec(spec, startedAt, e.tempDir)
	runner, err := e.backupRegistry.Runner(backupSpec.Type)
	if err != nil {
		e.reportRecordFailure(ctx, recordID, fmt.Sprintf("不支持的备份类型: %v", err))
		return err
	}

	// 3) 运行 runner
	logger := newRecordLogger(ctx, e.client, recordID)
	result, err := runner.Run(ctx, backupSpec, logger)
	if err != nil {
		e.reportRecordFailure(ctx, recordID, err.Error())
		return err
	}
	defer os.RemoveAll(result.TempDir)

	// 4) 可选 gzip 压缩
	finalPath := result.ArtifactPath
	if strings.EqualFold(spec.Compression, "gzip") && !strings.HasSuffix(strings.ToLower(finalPath), ".gz") {
		e.appendLog(ctx, recordID, "[agent] 开始压缩备份文件\n")
		compressedPath, compressErr := compress.GzipFile(finalPath)
		if compressErr != nil {
			e.reportRecordFailure(ctx, recordID, fmt.Sprintf("压缩失败: %v", compressErr))
			return compressErr
		}
		finalPath = compressedPath
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		e.reportRecordFailure(ctx, recordID, fmt.Sprintf("获取文件信息失败: %v", err))
		return err
	}
	fileName := filepath.Base(finalPath)
	fileSize := info.Size()
	storagePath := backup.BuildStorageKey(spec.Type, startedAt, fileName)

	// 5) 计算 checksum（一次读一次）并上传到所有目标
	checksum, err := computeFileSHA256(finalPath)
	if err != nil {
		e.reportRecordFailure(ctx, recordID, fmt.Sprintf("计算 checksum 失败: %v", err))
		return err
	}
	if len(spec.StorageTargets) == 0 {
		e.reportRecordFailure(ctx, recordID, "没有关联的存储目标")
		return fmt.Errorf("no storage targets")
	}
	for _, target := range spec.StorageTargets {
		if err := e.uploadToTarget(ctx, recordID, target, finalPath, storagePath, fileSize, spec.TaskID); err != nil {
			e.reportRecordFailure(ctx, recordID, fmt.Sprintf("上传到 %s 失败: %v", target.Name, err))
			return err
		}
		e.appendLog(ctx, recordID, fmt.Sprintf("[agent] 已上传到存储目标 %s\n", target.Name))
	}

	// 6) 上报最终成功
	return e.client.UpdateRecord(ctx, recordID, RecordUpdate{
		Status:      "success",
		FileName:    fileName,
		FileSize:    fileSize,
		Checksum:    checksum,
		StoragePath: storagePath,
		LogAppend:   fmt.Sprintf("[agent] 任务完成，总计 %d 字节\n", fileSize),
	})
}

// uploadToTarget 上传单个目标。为保持简化不做上传级重试（rclone 本身已有 low-level 重试）。
func (e *Executor) uploadToTarget(ctx context.Context, recordID uint, target StorageTargetConfig, filePath, objectKey string, fileSize int64, taskID uint) error {
	var rawConfig map[string]any
	if len(target.Config) > 0 {
		// DecodeRawConfig 通过 json 解析
		if err := jsonUnmarshalMap(target.Config, &rawConfig); err != nil {
			return fmt.Errorf("parse storage config: %w", err)
		}
	}
	provider, err := e.storageRegistry.Create(ctx, target.Type, rawConfig)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open artifact: %w", err)
	}
	defer f.Close()
	meta := map[string]string{
		"taskId":   fmt.Sprintf("%d", taskID),
		"recordId": fmt.Sprintf("%d", recordID),
	}
	return provider.Upload(ctx, objectKey, f, fileSize, meta)
}

// appendLog 追加日志到 Master 记录（尽力而为，失败不中断主流程）
func (e *Executor) appendLog(ctx context.Context, recordID uint, line string) {
	_ = e.client.UpdateRecord(ctx, recordID, RecordUpdate{LogAppend: line})
}

// reportRecordFailure 上报失败状态
func (e *Executor) reportRecordFailure(ctx context.Context, recordID uint, msg string) {
	_ = e.client.UpdateRecord(ctx, recordID, RecordUpdate{
		Status:       "failed",
		ErrorMessage: msg,
		LogAppend:    fmt.Sprintf("[agent] 错误: %s\n", msg),
	})
}

// buildBackupTaskSpec 把 AgentTaskSpec 转换为 backup.TaskSpec。
func buildBackupTaskSpec(spec *TaskSpec, startedAt time.Time, tempDir string) backup.TaskSpec {
	var sourcePaths []string
	if strings.TrimSpace(spec.SourcePaths) != "" {
		for _, p := range strings.Split(spec.SourcePaths, "\n") {
			if p = strings.TrimSpace(p); p != "" {
				sourcePaths = append(sourcePaths, p)
			}
		}
	}
	var excludes []string
	if strings.TrimSpace(spec.ExcludePatterns) != "" {
		for _, p := range strings.Split(spec.ExcludePatterns, "\n") {
			if p = strings.TrimSpace(p); p != "" {
				excludes = append(excludes, p)
			}
		}
	}
	return backup.TaskSpec{
		ID:              spec.TaskID,
		Name:            spec.Name,
		Type:            spec.Type,
		SourcePath:      spec.SourcePath,
		SourcePaths:     sourcePaths,
		ExcludePatterns: excludes,
		Database: backup.DatabaseSpec{
			Host:     spec.DBHost,
			Port:     spec.DBPort,
			User:     spec.DBUser,
			Password: spec.DBPassword,
			Path:     spec.DBPath,
			Names:    splitCommaOrNewline(spec.DBName),
		},
		Compression: spec.Compression,
		Encrypt:     spec.Encrypt,
		StartedAt:   startedAt,
		TempDir:     tempDir,
	}
}

// recordLogger 把 runner 日志回传到 Master 记录。
// 实现 backup.LogWriter，每条日志追加到 record.log_content。
type recordLogger struct {
	ctx      context.Context
	client   *MasterClient
	recordID uint
}

func newRecordLogger(ctx context.Context, client *MasterClient, recordID uint) *recordLogger {
	return &recordLogger{ctx: ctx, client: client, recordID: recordID}
}

func (l *recordLogger) WriteLine(message string) {
	_ = l.client.UpdateRecord(l.ctx, l.recordID, RecordUpdate{LogAppend: message + "\n"})
}

// restoreLogger 把 runner 日志回传到 Master 恢复记录。
type restoreLogger struct {
	ctx      context.Context
	client   *MasterClient
	restoreID uint
}

func newRestoreLogger(ctx context.Context, client *MasterClient, restoreID uint) *restoreLogger {
	return &restoreLogger{ctx: ctx, client: client, restoreID: restoreID}
}

func (l *restoreLogger) WriteLine(message string) {
	_ = l.client.UpdateRestore(l.ctx, l.restoreID, RestoreUpdate{LogAppend: message + "\n"})
}

// DeleteStorageObject 在 Agent 本机上删除指定存储对象（供跨节点清理调用）。
func (e *Executor) DeleteStorageObject(ctx context.Context, targetType string, targetConfig map[string]any, storagePath string) error {
	provider, err := e.storageRegistry.Create(ctx, targetType, targetConfig)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}
	return provider.Delete(ctx, storagePath)
}

// ExecuteRestore 处理 restore_record 命令：拉规格 → 下载 → 解压 → 执行 runner.Restore → 上报结果。
//
// 与 ExecuteRunTask 对称，但方向相反：
//   - 下载：通过 spec.Storage 创建 provider → Download(spec.StoragePath)
//   - 解密：当前 Agent 不支持加密恢复（密钥未下发），spec.Encrypt=true 会直接失败
//   - 执行：backup.Registry.Runner(spec.Type).Restore
//   - 上报：通过 UpdateRestore（status/logAppend）
func (e *Executor) ExecuteRestore(ctx context.Context, restoreRecordID uint) error {
	spec, err := e.client.GetRestoreSpec(ctx, restoreRecordID)
	if err != nil {
		e.reportRestoreFailure(ctx, restoreRecordID, fmt.Sprintf("拉取恢复规格失败: %v", err))
		return err
	}
	if spec.Encrypt {
		msg := "Agent 不支持加密恢复（加密密钥仅在 Master 端持有）"
		e.reportRestoreFailure(ctx, restoreRecordID, msg)
		return fmt.Errorf("%s", msg)
	}
	e.appendRestoreLog(ctx, restoreRecordID, fmt.Sprintf("[agent] 开始恢复 %s (type=%s)\n", spec.TaskName, spec.Type))

	if err := os.MkdirAll(e.tempDir, 0o755); err != nil {
		e.reportRestoreFailure(ctx, restoreRecordID, fmt.Sprintf("创建临时目录失败: %v", err))
		return err
	}
	tmpDir, err := os.MkdirTemp(e.tempDir, "restore-*")
	if err != nil {
		e.reportRestoreFailure(ctx, restoreRecordID, fmt.Sprintf("创建恢复临时目录失败: %v", err))
		return err
	}
	defer os.RemoveAll(tmpDir)

	// 1) 创建 storage provider
	var rawConfig map[string]any
	if len(spec.Storage.Config) > 0 {
		if err := jsonUnmarshalMap(spec.Storage.Config, &rawConfig); err != nil {
			e.reportRestoreFailure(ctx, restoreRecordID, fmt.Sprintf("解析存储配置失败: %v", err))
			return err
		}
	}
	provider, err := e.storageRegistry.Create(ctx, spec.Storage.Type, rawConfig)
	if err != nil {
		e.reportRestoreFailure(ctx, restoreRecordID, fmt.Sprintf("创建存储客户端失败: %v", err))
		return err
	}

	// 2) 下载
	fileName := spec.FileName
	if strings.TrimSpace(fileName) == "" {
		fileName = filepath.Base(spec.StoragePath)
	}
	artifactPath := filepath.Join(tmpDir, filepath.Base(fileName))
	e.appendRestoreLog(ctx, restoreRecordID, fmt.Sprintf("[agent] 下载备份文件 %s\n", spec.StoragePath))
	reader, err := provider.Download(ctx, spec.StoragePath)
	if err != nil {
		e.reportRestoreFailure(ctx, restoreRecordID, fmt.Sprintf("下载备份失败: %v", err))
		return err
	}
	if err := writeReaderToLocal(artifactPath, reader); err != nil {
		e.reportRestoreFailure(ctx, restoreRecordID, fmt.Sprintf("写入备份文件失败: %v", err))
		return err
	}

	// 3) 解压（Agent 不支持加密，遇到 .enc 会直接失败）
	preparedPath := artifactPath
	if strings.HasSuffix(strings.ToLower(preparedPath), ".enc") {
		msg := "检测到加密后缀，Agent 不支持加密恢复"
		e.reportRestoreFailure(ctx, restoreRecordID, msg)
		return fmt.Errorf("%s", msg)
	}
	if strings.HasSuffix(strings.ToLower(preparedPath), ".gz") {
		e.appendRestoreLog(ctx, restoreRecordID, "[agent] 解压 gzip 压缩\n")
		decompressed, err := compress.GunzipFile(preparedPath)
		if err != nil {
			e.reportRestoreFailure(ctx, restoreRecordID, fmt.Sprintf("解压失败: %v", err))
			return err
		}
		preparedPath = decompressed
	}

	// 4) 运行 runner.Restore
	taskSpec := buildRestoreBackupTaskSpec(spec, time.Now().UTC(), tmpDir)
	runner, err := e.backupRegistry.Runner(taskSpec.Type)
	if err != nil {
		e.reportRestoreFailure(ctx, restoreRecordID, fmt.Sprintf("不支持的备份类型: %v", err))
		return err
	}
	logger := newRestoreLogger(ctx, e.client, restoreRecordID)
	if err := runner.Restore(ctx, taskSpec, preparedPath, logger); err != nil {
		e.reportRestoreFailure(ctx, restoreRecordID, err.Error())
		return err
	}

	// 5) 上报成功
	return e.client.UpdateRestore(ctx, restoreRecordID, RestoreUpdate{
		Status:    "success",
		LogAppend: "[agent] 恢复执行完成\n",
	})
}

func (e *Executor) appendRestoreLog(ctx context.Context, restoreID uint, line string) {
	_ = e.client.UpdateRestore(ctx, restoreID, RestoreUpdate{LogAppend: line})
}

func (e *Executor) reportRestoreFailure(ctx context.Context, restoreID uint, msg string) {
	_ = e.client.UpdateRestore(ctx, restoreID, RestoreUpdate{
		Status:       "failed",
		ErrorMessage: msg,
		LogAppend:    fmt.Sprintf("[agent] 错误: %s\n", msg),
	})
}

// buildRestoreBackupTaskSpec 把 RestoreSpec 转成 backup.TaskSpec。
func buildRestoreBackupTaskSpec(spec *RestoreSpec, startedAt time.Time, tempDir string) backup.TaskSpec {
	return backup.TaskSpec{
		ID:              spec.TaskID,
		Name:            spec.TaskName,
		Type:            spec.Type,
		SourcePath:      spec.SourcePath,
		SourcePaths:     spec.SourcePaths,
		ExcludePatterns: nil,
		Database: backup.DatabaseSpec{
			Host:     spec.DBHost,
			Port:     spec.DBPort,
			User:     spec.DBUser,
			Password: spec.DBPassword,
			Path:     spec.DBPath,
			Names:    splitCommaOrNewline(spec.DBName),
		},
		Compression: spec.Compression,
		Encrypt:     spec.Encrypt,
		StartedAt:   startedAt,
		TempDir:     tempDir,
	}
}

// writeReaderToLocal 把 reader 写到本地文件（Agent 侧工具函数）。
func writeReaderToLocal(targetPath string, reader io.ReadCloser) error {
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

// 辅助函数

func computeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func splitCommaOrNewline(s string) []string {
	var result []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	}) {
		if p := strings.TrimSpace(part); p != "" {
			result = append(result, p)
		}
	}
	return result
}
