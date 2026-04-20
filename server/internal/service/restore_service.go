package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/backup"
	"backupx/server/internal/metrics"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
	"backupx/server/pkg/compress"
	backupcrypto "backupx/server/pkg/crypto"
)

// RestoreService 管理恢复记录生命周期并在集群中路由执行。
//
// 执行模型：
//   - task.NodeID == 0 或本机节点：Master 本地异步执行（runner.Restore），日志通过 LogHub 推到前端
//   - task.NodeID 指向远程节点：入队 AgentCommand("restore_record")，Agent 拉取 spec 后本地执行
//     并通过 HTTP 回传日志/状态，Master 再广播到 LogHub
type RestoreService struct {
	restores        repository.RestoreRecordRepository
	records         repository.BackupRecordRepository
	tasks           repository.BackupTaskRepository
	targets         repository.StorageTargetRepository
	nodeRepo        repository.NodeRepository
	storageRegistry *storage.Registry
	runnerRegistry  *backup.Registry
	logHub          *backup.LogHub
	cipher          *codec.ConfigCipher
	dispatcher      AgentDispatcher
	eventDispatcher EventDispatcher
	tempDir         string
	semaphore       chan struct{}
	async           func(func())
	now             func() time.Time
	metrics         *metrics.Metrics
}

// SetMetrics 注入 Prometheus 采集器。
func (s *RestoreService) SetMetrics(m *metrics.Metrics) {
	s.metrics = m
}

// NewRestoreService 构造恢复服务。maxConcurrent 控制本地并发恢复数。
func NewRestoreService(
	restores repository.RestoreRecordRepository,
	records repository.BackupRecordRepository,
	tasks repository.BackupTaskRepository,
	targets repository.StorageTargetRepository,
	nodeRepo repository.NodeRepository,
	storageRegistry *storage.Registry,
	runnerRegistry *backup.Registry,
	logHub *backup.LogHub,
	cipher *codec.ConfigCipher,
	dispatcher AgentDispatcher,
	tempDir string,
	maxConcurrent int,
) *RestoreService {
	if tempDir == "" {
		tempDir = "/tmp/backupx-restore"
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}
	return &RestoreService{
		restores:        restores,
		records:         records,
		tasks:           tasks,
		targets:         targets,
		nodeRepo:        nodeRepo,
		storageRegistry: storageRegistry,
		runnerRegistry:  runnerRegistry,
		logHub:          logHub,
		cipher:          cipher,
		dispatcher:      dispatcher,
		tempDir:         tempDir,
		semaphore:       make(chan struct{}, maxConcurrent),
		async:           func(job func()) { go job() },
		now:             func() time.Time { return time.Now().UTC() },
	}
}

// SetEventDispatcher 注入事件分发通道，用于恢复完成/失败的 Webhook 派发。
func (s *RestoreService) SetEventDispatcher(dispatcher EventDispatcher) {
	s.eventDispatcher = dispatcher
}

// RestoreRecordSummary 列表项。
type RestoreRecordSummary struct {
	ID              uint       `json:"id"`
	BackupRecordID  uint       `json:"backupRecordId"`
	TaskID          uint       `json:"taskId"`
	TaskName        string     `json:"taskName"`
	NodeID          uint       `json:"nodeId"`
	NodeName        string     `json:"nodeName,omitempty"`
	Status          string     `json:"status"`
	ErrorMessage    string     `json:"errorMessage"`
	DurationSeconds int        `json:"durationSeconds"`
	StartedAt       time.Time  `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	TriggeredBy     string     `json:"triggeredBy"`
	BackupFileName  string     `json:"backupFileName,omitempty"`
}

// RestoreRecordDetail 详情（含日志）。
type RestoreRecordDetail struct {
	RestoreRecordSummary
	LogContent string            `json:"logContent"`
	LogEvents  []backup.LogEvent `json:"logEvents,omitempty"`
}

// Start 触发一次恢复。返回新建 RestoreRecord 详情。
// 若任务绑定远程节点：入队 AgentCommand 后立即返回（状态为 running）
// 若本地：异步执行并立即返回。
func (s *RestoreService) Start(ctx context.Context, backupRecordID uint, triggeredBy string) (*RestoreRecordDetail, error) {
	record, err := s.records.FindByID(ctx, backupRecordID)
	if err != nil {
		return nil, apperror.Internal("BACKUP_RECORD_GET_FAILED", "无法获取备份记录", err)
	}
	if record == nil {
		return nil, apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "备份记录不存在", fmt.Errorf("backup record %d not found", backupRecordID))
	}
	if record.Status != model.BackupRecordStatusSuccess {
		return nil, apperror.BadRequest("RESTORE_SOURCE_INVALID", "只能恢复状态为成功的备份记录", nil)
	}
	task, err := s.tasks.FindByID(ctx, record.TaskID)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_GET_FAILED", "无法获取关联备份任务", err)
	}
	if task == nil {
		return nil, apperror.New(404, "BACKUP_TASK_NOT_FOUND", "关联的备份任务不存在", fmt.Errorf("backup task %d not found", record.TaskID))
	}

	startedAt := s.now()
	restore := &model.RestoreRecord{
		BackupRecordID: backupRecordID,
		TaskID:         record.TaskID,
		NodeID:         task.NodeID,
		Status:         model.RestoreRecordStatusRunning,
		StartedAt:      startedAt,
		TriggeredBy:    strings.TrimSpace(triggeredBy),
	}
	if err := s.restores.Create(ctx, restore); err != nil {
		return nil, apperror.Internal("RESTORE_RECORD_CREATE_FAILED", "无法创建恢复记录", err)
	}

	// 远程节点路由
	if remoteNode := s.resolveRemoteNode(ctx, task.NodeID); remoteNode != nil {
		if s.dispatcher == nil {
			return nil, apperror.Internal("RESTORE_DISPATCH_UNAVAILABLE", "Agent 下发通道未就绪", nil)
		}
		// 节点离线 → 立即标记 failed，避免记录永远卡在 running
		if remoteNode.Status != model.NodeStatusOnline {
			offlineMsg := fmt.Sprintf("节点 %s 当前离线，无法执行恢复", remoteNode.Name)
			_ = s.finalize(ctx, restore.ID, model.RestoreRecordStatusFailed, offlineMsg)
			s.logHub.Append(restore.ID, "error", offlineMsg)
			s.logHub.Complete(restore.ID, model.RestoreRecordStatusFailed)
			return nil, apperror.BadRequest("NODE_OFFLINE", offlineMsg, nil)
		}
		if _, dispatchErr := s.dispatcher.EnqueueCommand(ctx, task.NodeID, model.AgentCommandTypeRestoreRecord, map[string]any{
			"restoreRecordId": restore.ID,
		}); dispatchErr != nil {
			_ = s.finalize(ctx, restore.ID, model.RestoreRecordStatusFailed,
				"下发恢复任务到远程节点失败: "+dispatchErr.Error())
			return nil, apperror.Internal("AGENT_COMMAND_ENQUEUE_FAILED", "无法下发恢复任务到远程节点", dispatchErr)
		}
		s.logHub.Append(restore.ID, "info", fmt.Sprintf("已下发恢复任务到节点 %s（#%d），等待 Agent 执行", remoteNode.Name, task.NodeID))
		return s.getDetail(ctx, restore.ID)
	}

	// 本地节点：异步执行
	run := func() {
		s.executeLocally(context.Background(), restore.ID, task, record)
	}
	s.async(run)
	return s.getDetail(ctx, restore.ID)
}

// isRemoteNode 判断 NodeID 是否指向有效的远程节点。
func (s *RestoreService) isRemoteNode(ctx context.Context, nodeID uint) bool {
	return s.resolveRemoteNode(ctx, nodeID) != nil
}

// resolveRemoteNode 返回远程节点指针（含 Status），用于离线判定。
func (s *RestoreService) resolveRemoteNode(ctx context.Context, nodeID uint) *model.Node {
	if s.nodeRepo == nil || s.dispatcher == nil || nodeID == 0 {
		return nil
	}
	node, err := s.nodeRepo.FindByID(ctx, nodeID)
	if err != nil || node == nil || node.IsLocal {
		return nil
	}
	return node
}

// executeLocally 在 Master 本地执行恢复。
func (s *RestoreService) executeLocally(ctx context.Context, restoreID uint, task *model.BackupTask, backupRecord *model.BackupRecord) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	logger := backup.NewExecutionLogger(restoreID, s.logHub)
	status := model.RestoreRecordStatusFailed
	errMessage := ""

	defer func() {
		finalizeErr := s.finalizeWithLog(ctx, restoreID, status, errMessage, logger.String())
		if finalizeErr != nil {
			logger.Errorf("写回恢复记录失败：%v", finalizeErr)
		}
		s.logHub.Complete(restoreID, status)
		s.dispatchRestoreEvent(ctx, restoreID, status, errMessage, task)
	}()

	logger.Infof("开始在本地执行恢复（备份记录 #%d）", backupRecord.ID)
	provider, providerErr := s.resolveProvider(ctx, backupRecord.StorageTargetID)
	if providerErr != nil {
		errMessage = providerErr.Error()
		logger.Errorf("创建存储客户端失败：%v", providerErr)
		return
	}

	if err := os.MkdirAll(s.tempDir, 0o755); err != nil {
		errMessage = err.Error()
		logger.Errorf("创建恢复临时父目录失败：%v", err)
		return
	}
	tempDir, tempErr := os.MkdirTemp(s.tempDir, "restore-*")
	if tempErr != nil {
		errMessage = tempErr.Error()
		logger.Errorf("创建恢复临时目录失败：%v", tempErr)
		return
	}
	defer os.RemoveAll(tempDir)

	fileName := backupRecord.FileName
	if strings.TrimSpace(fileName) == "" {
		fileName = filepath.Base(backupRecord.StoragePath)
	}
	artifactPath := filepath.Join(tempDir, filepath.Base(fileName))
	logger.Infof("开始下载备份文件：%s", backupRecord.StoragePath)
	reader, downloadErr := provider.Download(ctx, backupRecord.StoragePath)
	if downloadErr != nil {
		errMessage = downloadErr.Error()
		logger.Errorf("下载备份文件失败：%v", downloadErr)
		return
	}
	if writeErr := writeReaderToFile(artifactPath, reader); writeErr != nil {
		errMessage = writeErr.Error()
		logger.Errorf("写入恢复文件失败：%v", writeErr)
		return
	}
	preparedPath, prepareErr := s.prepareArtifact(artifactPath, logger)
	if prepareErr != nil {
		errMessage = prepareErr.Error()
		logger.Errorf("准备恢复文件失败：%v", prepareErr)
		return
	}

	spec, specErr := s.buildTaskSpec(task, backupRecord.StartedAt)
	if specErr != nil {
		errMessage = specErr.Error()
		logger.Errorf("构建恢复规格失败：%v", specErr)
		return
	}
	runner, runnerErr := s.runnerRegistry.Runner(spec.Type)
	if runnerErr != nil {
		errMessage = runnerErr.Error()
		logger.Errorf("不支持的备份类型：%v", runnerErr)
		return
	}
	logger.Infof("开始执行 %s 恢复", spec.Type)
	if restoreErr := runner.Restore(ctx, spec, preparedPath, logger); restoreErr != nil {
		errMessage = restoreErr.Error()
		logger.Errorf("恢复执行失败：%v", restoreErr)
		return
	}
	status = model.RestoreRecordStatusSuccess
	logger.Infof("恢复执行成功")
}

// dispatchRestoreEvent 按终态向事件总线派发 restore_success 或 restore_failed。
// eventDispatcher 未注入时静默忽略，保持向后兼容。
func (s *RestoreService) dispatchRestoreEvent(ctx context.Context, restoreID uint, status, errMessage string, task *model.BackupTask) {
	if s.eventDispatcher == nil {
		return
	}
	var eventType, title string
	switch status {
	case model.RestoreRecordStatusSuccess:
		eventType = model.NotificationEventRestoreSuccess
		title = "BackupX 恢复成功"
	case model.RestoreRecordStatusFailed:
		eventType = model.NotificationEventRestoreFailed
		title = "BackupX 恢复失败"
	default:
		return
	}
	taskName := "未知任务"
	if task != nil {
		taskName = task.Name
	}
	body := fmt.Sprintf("任务：%s\n恢复记录：#%d\n状态：%s", taskName, restoreID, status)
	if errMessage != "" {
		body += "\n错误：" + errMessage
	}
	fields := map[string]any{
		"restoreId": restoreID,
		"taskName":  taskName,
		"status":    status,
		"error":     errMessage,
	}
	if task != nil {
		fields["taskId"] = task.ID
	}
	_ = s.eventDispatcher.DispatchEvent(ctx, eventType, title, body, fields)
}

// resolveProvider 复用 BackupExecutionService 的逻辑（解密 → 创建 provider）。
func (s *RestoreService) resolveProvider(ctx context.Context, targetID uint) (storage.StorageProvider, error) {
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
	return s.storageRegistry.Create(ctx, target.Type, configMap)
}

// prepareArtifact 根据文件后缀依次解密、解压。
func (s *RestoreService) prepareArtifact(artifactPath string, logger *backup.ExecutionLogger) (string, error) {
	currentPath := artifactPath
	if strings.HasSuffix(strings.ToLower(currentPath), ".enc") {
		logger.Infof("检测到加密后缀，开始解密")
		decrypted, err := backupcrypto.DecryptFile(s.cipher.Key(), currentPath)
		if err != nil {
			return "", err
		}
		currentPath = decrypted
	}
	if strings.HasSuffix(strings.ToLower(currentPath), ".gz") {
		logger.Infof("检测到 gzip 压缩，开始解压")
		decompressed, err := compress.GunzipFile(currentPath)
		if err != nil {
			return "", err
		}
		currentPath = decompressed
	}
	return currentPath, nil
}

// buildTaskSpec 复刻 BackupExecutionService.buildTaskSpec 的核心逻辑。
func (s *RestoreService) buildTaskSpec(task *model.BackupTask, startedAt time.Time) (backup.TaskSpec, error) {
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
	if strings.TrimSpace(task.ExtraConfig) != "" {
		extra := map[string]any{}
		if err := json.Unmarshal([]byte(task.ExtraConfig), &extra); err != nil {
			return backup.TaskSpec{}, apperror.Internal("BACKUP_TASK_DECODE_FAILED", "无法解析扩展配置", err)
		}
		applyHANAExtraConfig(&dbSpec, extra)
	}
	return backup.TaskSpec{
		ID:              task.ID,
		Name:            task.Name,
		Type:            task.Type,
		SourcePath:      task.SourcePath,
		SourcePaths:     sourcePaths,
		ExcludePatterns: excludePatterns,
		StorageTargetID: task.StorageTargetID,
		Compression:     task.Compression,
		Encrypt:         task.Encrypt,
		RetentionDays:   task.RetentionDays,
		MaxBackups:      task.MaxBackups,
		StartedAt:       startedAt,
		TempDir:         s.tempDir,
		Database:        dbSpec,
	}, nil
}

// finalize 只更新状态和错误信息，不写 log（用于失败的 dispatch 路径）。
func (s *RestoreService) finalize(ctx context.Context, restoreID uint, status, errMessage string) error {
	return s.finalizeWithLog(ctx, restoreID, status, errMessage, "")
}

// finalizeWithLog 把恢复记录写成终态。
func (s *RestoreService) finalizeWithLog(ctx context.Context, restoreID uint, status, errMessage, logContent string) error {
	record, err := s.restores.FindByID(ctx, restoreID)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("restore record %d not found", restoreID)
	}
	completedAt := s.now()
	record.Status = status
	record.ErrorMessage = strings.TrimSpace(errMessage)
	if strings.TrimSpace(logContent) != "" {
		record.LogContent = strings.TrimSpace(logContent)
	}
	record.DurationSeconds = int(completedAt.Sub(record.StartedAt).Seconds())
	record.CompletedAt = &completedAt
	s.metrics.ObserveRestore(status)
	return s.restores.Update(ctx, record)
}

// Get 查恢复记录详情。
func (s *RestoreService) Get(ctx context.Context, restoreID uint) (*RestoreRecordDetail, error) {
	return s.getDetail(ctx, restoreID)
}

// List 列表。
func (s *RestoreService) List(ctx context.Context, input RestoreRecordListInput) ([]RestoreRecordSummary, error) {
	items, err := s.restores.List(ctx, repository.RestoreRecordListOptions{
		TaskID:         input.TaskID,
		BackupRecordID: input.BackupRecordID,
		NodeID:         input.NodeID,
		Status:         strings.TrimSpace(input.Status),
		DateFrom:       input.DateFrom,
		DateTo:         input.DateTo,
		Limit:          input.Limit,
		Offset:         input.Offset,
	})
	if err != nil {
		return nil, apperror.Internal("RESTORE_RECORD_LIST_FAILED", "无法获取恢复记录列表", err)
	}
	result := make([]RestoreRecordSummary, 0, len(items))
	nodeNames := map[uint]string{}
	for _, item := range items {
		nodeName := ""
		if item.NodeID > 0 && s.nodeRepo != nil {
			if cached, ok := nodeNames[item.NodeID]; ok {
				nodeName = cached
			} else if node, err := s.nodeRepo.FindByID(ctx, item.NodeID); err == nil && node != nil {
				nodeName = node.Name
				nodeNames[item.NodeID] = node.Name
			}
		}
		result = append(result, toRestoreRecordSummary(&item, nodeName))
	}
	return result, nil
}

// SubscribeLogs 订阅指定恢复记录的实时日志。
func (s *RestoreService) SubscribeLogs(ctx context.Context, restoreID uint, buffer int) (<-chan backup.LogEvent, func(), error) {
	record, err := s.restores.FindByID(ctx, restoreID)
	if err != nil {
		return nil, nil, apperror.Internal("RESTORE_RECORD_GET_FAILED", "无法获取恢复记录详情", err)
	}
	if record == nil {
		return nil, nil, apperror.New(404, "RESTORE_RECORD_NOT_FOUND", "恢复记录不存在", nil)
	}
	channel, cancel := s.logHub.Subscribe(restoreID, buffer)
	return channel, cancel, nil
}

// RestoreRecordListInput 列表查询参数。
type RestoreRecordListInput struct {
	TaskID         *uint
	BackupRecordID *uint
	NodeID         *uint
	Status         string
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}

// --- Agent 侧调用接口 ---

// AgentRestoreSpec 下发给 Agent 执行恢复的完整规格。
type AgentRestoreSpec struct {
	RestoreRecordID uint                     `json:"restoreRecordId"`
	BackupRecordID  uint                     `json:"backupRecordId"`
	TaskID          uint                     `json:"taskId"`
	TaskName        string                   `json:"taskName"`
	Type            string                   `json:"type"`
	SourcePath      string                   `json:"sourcePath,omitempty"`
	SourcePaths     []string                 `json:"sourcePaths,omitempty"`
	DBHost          string                   `json:"dbHost,omitempty"`
	DBPort          int                      `json:"dbPort,omitempty"`
	DBUser          string                   `json:"dbUser,omitempty"`
	DBPassword      string                   `json:"dbPassword,omitempty"`
	DBName          string                   `json:"dbName,omitempty"`
	DBPath          string                   `json:"dbPath,omitempty"`
	ExtraConfig     string                   `json:"extraConfig,omitempty"`
	Compression     string                   `json:"compression"`
	Encrypt         bool                     `json:"encrypt"`
	Storage         AgentStorageTargetConfig `json:"storage"`
	StoragePath     string                   `json:"storagePath"`
	FileName        string                   `json:"fileName"`
}

// AgentRestoreUpdate Agent 回传的增量更新。
type AgentRestoreUpdate struct {
	Status       string `json:"status,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	LogAppend    string `json:"logAppend,omitempty"`
}

// GetAgentRestoreSpec 供 Agent 拉取恢复规格。需校验恢复记录属于当前节点。
func (s *RestoreService) GetAgentRestoreSpec(ctx context.Context, node *model.Node, restoreID uint) (*AgentRestoreSpec, error) {
	restore, err := s.restores.FindByID(ctx, restoreID)
	if err != nil {
		return nil, err
	}
	if restore == nil {
		return nil, apperror.New(404, "RESTORE_RECORD_NOT_FOUND", "恢复记录不存在", nil)
	}
	if restore.NodeID != node.ID {
		return nil, apperror.Unauthorized("RESTORE_RECORD_FORBIDDEN", "恢复记录不属于当前节点", nil)
	}
	backupRecord, err := s.records.FindByID(ctx, restore.BackupRecordID)
	if err != nil {
		return nil, err
	}
	if backupRecord == nil {
		return nil, apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "源备份记录不存在", nil)
	}
	task, err := s.tasks.FindByID(ctx, restore.TaskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, apperror.New(404, "BACKUP_TASK_NOT_FOUND", "备份任务不存在", nil)
	}
	// 解密数据库密码
	dbPassword := ""
	if strings.TrimSpace(task.DBPasswordCiphertext) != "" {
		plain, decErr := s.cipher.Decrypt(task.DBPasswordCiphertext)
		if decErr != nil {
			return nil, fmt.Errorf("decrypt db password: %w", decErr)
		}
		dbPassword = string(plain)
	}
	// 解密备份时使用的存储目标
	target, err := s.targets.FindByID(ctx, backupRecord.StorageTargetID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, apperror.BadRequest("BACKUP_STORAGE_TARGET_INVALID", "存储目标不存在", nil)
	}
	configRaw, err := s.cipher.Decrypt(target.ConfigCiphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt storage config: %w", err)
	}
	// 拆开 sourcePaths
	sourcePaths := []string{}
	if strings.TrimSpace(task.SourcePaths) != "" {
		_ = json.Unmarshal([]byte(task.SourcePaths), &sourcePaths)
	}
	return &AgentRestoreSpec{
		RestoreRecordID: restore.ID,
		BackupRecordID:  backupRecord.ID,
		TaskID:          task.ID,
		TaskName:        task.Name,
		Type:            task.Type,
		SourcePath:      task.SourcePath,
		SourcePaths:     sourcePaths,
		DBHost:          task.DBHost,
		DBPort:          task.DBPort,
		DBUser:          task.DBUser,
		DBPassword:      dbPassword,
		DBName:          task.DBName,
		DBPath:          task.DBPath,
		ExtraConfig:     task.ExtraConfig,
		Compression:     task.Compression,
		Encrypt:         task.Encrypt,
		Storage: AgentStorageTargetConfig{
			ID:     target.ID,
			Type:   target.Type,
			Name:   target.Name,
			Config: json.RawMessage(configRaw),
		},
		StoragePath: backupRecord.StoragePath,
		FileName:    backupRecord.FileName,
	}, nil
}

// UpdateAgentRestore Agent 回传状态/日志。
func (s *RestoreService) UpdateAgentRestore(ctx context.Context, node *model.Node, restoreID uint, update AgentRestoreUpdate) error {
	restore, err := s.restores.FindByID(ctx, restoreID)
	if err != nil {
		return err
	}
	if restore == nil {
		return apperror.New(404, "RESTORE_RECORD_NOT_FOUND", "恢复记录不存在", nil)
	}
	if restore.NodeID != node.ID {
		return apperror.Unauthorized("RESTORE_RECORD_FORBIDDEN", "恢复记录不属于当前节点", nil)
	}
	// 追加日志到 LogHub + DB
	if strings.TrimSpace(update.LogAppend) != "" {
		for _, line := range strings.Split(update.LogAppend, "\n") {
			trimmed := strings.TrimRight(line, "\r")
			if strings.TrimSpace(trimmed) == "" {
				continue
			}
			s.logHub.Append(restoreID, "info", trimmed)
		}
		if strings.TrimSpace(restore.LogContent) == "" {
			restore.LogContent = update.LogAppend
		} else {
			if !strings.HasSuffix(restore.LogContent, "\n") {
				restore.LogContent += "\n"
			}
			restore.LogContent += update.LogAppend
		}
	}
	if update.Status != "" {
		restore.Status = update.Status
		if update.Status == model.RestoreRecordStatusSuccess || update.Status == model.RestoreRecordStatusFailed {
			completedAt := s.now()
			restore.CompletedAt = &completedAt
			restore.DurationSeconds = int(completedAt.Sub(restore.StartedAt).Seconds())
			if strings.TrimSpace(update.ErrorMessage) != "" {
				restore.ErrorMessage = strings.TrimSpace(update.ErrorMessage)
			}
		}
	}
	if err := s.restores.Update(ctx, restore); err != nil {
		return err
	}
	if update.Status == model.RestoreRecordStatusSuccess || update.Status == model.RestoreRecordStatusFailed {
		s.logHub.Complete(restoreID, update.Status)
	}
	return nil
}

// --- 内部辅助 ---

func (s *RestoreService) getDetail(ctx context.Context, restoreID uint) (*RestoreRecordDetail, error) {
	record, err := s.restores.FindByID(ctx, restoreID)
	if err != nil {
		return nil, apperror.Internal("RESTORE_RECORD_GET_FAILED", "无法获取恢复记录详情", err)
	}
	if record == nil {
		return nil, apperror.New(404, "RESTORE_RECORD_NOT_FOUND", "恢复记录不存在", nil)
	}
	nodeName := ""
	if record.NodeID > 0 && s.nodeRepo != nil {
		if node, err := s.nodeRepo.FindByID(ctx, record.NodeID); err == nil && node != nil {
			nodeName = node.Name
		}
	}
	detail := &RestoreRecordDetail{
		RestoreRecordSummary: toRestoreRecordSummary(record, nodeName),
		LogContent:           record.LogContent,
	}
	if record.Status == model.RestoreRecordStatusRunning && s.logHub != nil {
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

func toRestoreRecordSummary(item *model.RestoreRecord, nodeName string) RestoreRecordSummary {
	summary := RestoreRecordSummary{
		ID:              item.ID,
		BackupRecordID:  item.BackupRecordID,
		TaskID:          item.TaskID,
		TaskName:        item.Task.Name,
		NodeID:          item.NodeID,
		NodeName:        nodeName,
		Status:          item.Status,
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
