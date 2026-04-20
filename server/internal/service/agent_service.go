package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage/codec"
)

// AgentService 实现 Master 端 Agent 协议，提供给远程 Agent 通过 HTTP 调用。
// 所有方法使用 Agent Token 进行节点认证，避免暴露 JWT 给 Agent。
type AgentService struct {
	nodeRepo    repository.NodeRepository
	taskRepo    repository.BackupTaskRepository
	recordRepo  repository.BackupRecordRepository
	storageRepo repository.StorageTargetRepository
	cmdRepo     repository.AgentCommandRepository
	restoreRepo repository.RestoreRecordRepository
	cipher      *codec.ConfigCipher
}

func NewAgentService(
	nodeRepo repository.NodeRepository,
	taskRepo repository.BackupTaskRepository,
	recordRepo repository.BackupRecordRepository,
	storageRepo repository.StorageTargetRepository,
	cmdRepo repository.AgentCommandRepository,
	cipher *codec.ConfigCipher,
) *AgentService {
	return &AgentService{
		nodeRepo:    nodeRepo,
		taskRepo:    taskRepo,
		recordRepo:  recordRepo,
		storageRepo: storageRepo,
		cmdRepo:     cmdRepo,
		cipher:      cipher,
	}
}

// SetRestoreRepository 注入恢复记录仓储，用于命令超时时联动 restore_record 状态。
// 可选注入：未注入时恢复命令超时仅标记命令 timeout，记录需另行查验。
func (s *AgentService) SetRestoreRepository(repo repository.RestoreRecordRepository) {
	s.restoreRepo = repo
}

// AuthenticatedNode 通过 token 解析并返回节点。失败返回 401。
func (s *AgentService) AuthenticatedNode(ctx context.Context, token string) (*model.Node, error) {
	if strings.TrimSpace(token) == "" {
		return nil, apperror.Unauthorized("NODE_INVALID_TOKEN", "缺少认证令牌", nil)
	}
	node, err := s.nodeRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, apperror.Unauthorized("NODE_INVALID_TOKEN", "无效的节点认证令牌", nil)
	}
	return node, nil
}

// AgentCommandPayload 给 Agent 返回的命令描述
type AgentCommandPayload struct {
	ID      uint            `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// PollCommand 为指定节点拉取一条 pending 命令；无命令时返回 (nil, nil)。
func (s *AgentService) PollCommand(ctx context.Context, node *model.Node) (*AgentCommandPayload, error) {
	cmd, err := s.cmdRepo.ClaimPending(ctx, node.ID)
	if err != nil {
		return nil, err
	}
	if cmd == nil {
		return nil, nil
	}
	return &AgentCommandPayload{
		ID:      cmd.ID,
		Type:    cmd.Type,
		Payload: json.RawMessage(cmd.Payload),
	}, nil
}

// AgentCommandResult Agent 上报命令执行结果
type AgentCommandResult struct {
	Success      bool            `json:"success"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
}

// SubmitCommandResult 接收 Agent 上报的命令结果。
func (s *AgentService) SubmitCommandResult(ctx context.Context, node *model.Node, cmdID uint, result AgentCommandResult) error {
	cmd, err := s.cmdRepo.FindByID(ctx, cmdID)
	if err != nil {
		return err
	}
	if cmd == nil {
		return apperror.New(404, "AGENT_COMMAND_NOT_FOUND", "命令不存在", fmt.Errorf("command %d not found", cmdID))
	}
	if cmd.NodeID != node.ID {
		return apperror.Unauthorized("AGENT_COMMAND_FORBIDDEN", "命令不属于当前节点", nil)
	}
	now := time.Now().UTC()
	if result.Success {
		cmd.Status = model.AgentCommandStatusSucceeded
	} else {
		cmd.Status = model.AgentCommandStatusFailed
	}
	cmd.ErrorMessage = result.ErrorMessage
	if len(result.Result) > 0 {
		cmd.Result = string(result.Result)
	}
	cmd.CompletedAt = &now
	return s.cmdRepo.Update(ctx, cmd)
}

// AgentTaskSpec 给 Agent 返回的任务规格，包含解密后的存储配置，供 Agent 直接执行。
// 敏感信息：此接口仅供 Agent 调用（token 认证），避免通过公共 API 泄露。
type AgentTaskSpec struct {
	TaskID          uint                       `json:"taskId"`
	Name            string                     `json:"name"`
	Type            string                     `json:"type"`
	SourcePath      string                     `json:"sourcePath,omitempty"`
	SourcePaths     string                     `json:"sourcePaths,omitempty"`
	ExcludePatterns string                     `json:"excludePatterns,omitempty"`
	DBHost          string                     `json:"dbHost,omitempty"`
	DBPort          int                        `json:"dbPort,omitempty"`
	DBUser          string                     `json:"dbUser,omitempty"`
	DBPassword      string                     `json:"dbPassword,omitempty"`
	DBName          string                     `json:"dbName,omitempty"`
	DBPath          string                     `json:"dbPath,omitempty"`
	ExtraConfig     string                     `json:"extraConfig,omitempty"`
	Compression     string                     `json:"compression"`
	Encrypt         bool                       `json:"encrypt"`
	StorageTargets  []AgentStorageTargetConfig `json:"storageTargets"`
}

// AgentStorageTargetConfig 存储目标配置（已解密）
type AgentStorageTargetConfig struct {
	ID     uint            `json:"id"`
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

// GetTaskSpec 返回 Agent 执行任务所需的完整规格。
func (s *AgentService) GetTaskSpec(ctx context.Context, node *model.Node, taskID uint) (*AgentTaskSpec, error) {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, apperror.New(404, "BACKUP_TASK_NOT_FOUND", "任务不存在", nil)
	}
	if task.NodeID != node.ID {
		return nil, apperror.Unauthorized("BACKUP_TASK_FORBIDDEN", "任务不属于当前节点", nil)
	}
	// 解密数据库密码（若有）
	dbPassword := ""
	if task.DBPasswordCiphertext != "" {
		plain, decErr := s.cipher.Decrypt(task.DBPasswordCiphertext)
		if decErr != nil {
			return nil, fmt.Errorf("decrypt db password: %w", decErr)
		}
		dbPassword = string(plain)
	}
	// 解密存储目标配置
	targets := collectTargetIDs(task)
	storageTargets := make([]AgentStorageTargetConfig, 0, len(targets))
	for _, tid := range targets {
		target, err := s.storageRepo.FindByID(ctx, tid)
		if err != nil {
			return nil, err
		}
		if target == nil {
			continue
		}
		configRaw, err := s.cipher.Decrypt(target.ConfigCiphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt storage config: %w", err)
		}
		storageTargets = append(storageTargets, AgentStorageTargetConfig{
			ID:     target.ID,
			Type:   target.Type,
			Name:   target.Name,
			Config: json.RawMessage(configRaw),
		})
	}
	return &AgentTaskSpec{
		TaskID:          task.ID,
		Name:            task.Name,
		Type:            task.Type,
		SourcePath:      task.SourcePath,
		SourcePaths:     task.SourcePaths,
		ExcludePatterns: task.ExcludePatterns,
		DBHost:          task.DBHost,
		DBPort:          task.DBPort,
		DBUser:          task.DBUser,
		DBPassword:      dbPassword,
		DBName:          task.DBName,
		DBPath:          task.DBPath,
		ExtraConfig:     task.ExtraConfig,
		Compression:     task.Compression,
		Encrypt:         task.Encrypt,
		StorageTargets:  storageTargets,
	}, nil
}

// AgentRecordUpdate Agent 上报备份记录的最终状态。
type AgentRecordUpdate struct {
	Status       string `json:"status"` // running | success | failed
	FileName     string `json:"fileName,omitempty"`
	FileSize     int64  `json:"fileSize,omitempty"`
	Checksum     string `json:"checksum,omitempty"`
	StoragePath  string `json:"storagePath,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	LogAppend    string `json:"logAppend,omitempty"` // 增量日志，追加到 record.log_content
}

// UpdateRecord 更新备份记录的状态/日志。Agent 在执行过程中可多次调用。
func (s *AgentService) UpdateRecord(ctx context.Context, node *model.Node, recordID uint, update AgentRecordUpdate) error {
	record, err := s.recordRepo.FindByID(ctx, recordID)
	if err != nil {
		return err
	}
	if record == nil {
		return apperror.New(404, "BACKUP_RECORD_NOT_FOUND", "记录不存在", nil)
	}
	// 通过 task.NodeID 判断是否属于当前 agent
	task, err := s.taskRepo.FindByID(ctx, record.TaskID)
	if err != nil {
		return err
	}
	if task == nil || task.NodeID != node.ID {
		return apperror.Unauthorized("BACKUP_RECORD_FORBIDDEN", "记录不属于当前节点", nil)
	}
	if update.Status != "" {
		record.Status = update.Status
	}
	if update.FileName != "" {
		record.FileName = update.FileName
	}
	if update.FileSize > 0 {
		record.FileSize = update.FileSize
	}
	if update.Checksum != "" {
		record.Checksum = update.Checksum
	}
	if update.StoragePath != "" {
		record.StoragePath = update.StoragePath
	}
	if update.ErrorMessage != "" {
		record.ErrorMessage = update.ErrorMessage
	}
	if update.LogAppend != "" {
		if record.LogContent == "" {
			record.LogContent = update.LogAppend
		} else {
			record.LogContent += update.LogAppend
		}
	}
	if update.Status == model.BackupRecordStatusSuccess || update.Status == model.BackupRecordStatusFailed {
		now := time.Now().UTC()
		record.CompletedAt = &now
		record.DurationSeconds = int(now.Sub(record.StartedAt).Seconds())
	}
	if err := s.recordRepo.Update(ctx, record); err != nil {
		return err
	}
	// 同步更新任务的 last_status
	if update.Status == model.BackupRecordStatusSuccess || update.Status == model.BackupRecordStatusFailed {
		task.LastStatus = update.Status
		_ = s.taskRepo.Update(ctx, task)
	}
	return nil
}

// EnqueueCommand Master 端调用：给指定节点插入一条待执行命令。
// 返回命令 ID。
func (s *AgentService) EnqueueCommand(ctx context.Context, nodeID uint, cmdType string, payload any) (uint, error) {
	if nodeID == 0 {
		return 0, errors.New("nodeID is required")
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}
	cmd := &model.AgentCommand{
		NodeID:  nodeID,
		Type:    cmdType,
		Status:  model.AgentCommandStatusPending,
		Payload: string(payloadBytes),
	}
	if err := s.cmdRepo.Create(ctx, cmd); err != nil {
		return 0, err
	}
	return cmd.ID, nil
}

// WaitForCommandResult 同步等待指定命令完成（用于 list_dir 这类 RPC 式调用）。
// timeout 为 0 表示不限，建议传 10~30s。
func (s *AgentService) WaitForCommandResult(ctx context.Context, cmdID uint, timeout time.Duration) (*model.AgentCommand, error) {
	deadline := time.Now().Add(timeout)
	for {
		cmd, err := s.cmdRepo.FindByID(ctx, cmdID)
		if err != nil {
			return nil, err
		}
		if cmd == nil {
			return nil, apperror.New(404, "AGENT_COMMAND_NOT_FOUND", "命令不存在", nil)
		}
		switch cmd.Status {
		case model.AgentCommandStatusSucceeded, model.AgentCommandStatusFailed, model.AgentCommandStatusTimeout:
			return cmd, nil
		}
		if timeout > 0 && time.Now().After(deadline) {
			return nil, apperror.New(504, "AGENT_COMMAND_TIMEOUT", "等待 Agent 响应超时", nil)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// StartCommandTimeoutMonitor 启动后台定时任务，把超时命令标记为 timeout。
// 对于 run_task / restore_record 命令，同时把关联的 BackupRecord / RestoreRecord
// 标记为 failed，避免 Agent 离线/崩溃时记录永远卡在 running。
func (s *AgentService) StartCommandTimeoutMonitor(ctx context.Context, interval time.Duration, timeout time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				threshold := time.Now().UTC().Add(-timeout)
				s.processStaleCommands(ctx, threshold)
			}
		}
	}()
}

// processStaleCommands 扫描已超时的 dispatched 命令并联动关联记录。
// 流程：先取超时候选 → 对每条联动 backup/restore 记录 → 把命令置为 timeout。
// 单条失败不影响后续处理。
func (s *AgentService) processStaleCommands(ctx context.Context, threshold time.Time) {
	commands, err := s.cmdRepo.ListStaleDispatched(ctx, threshold)
	if err != nil || len(commands) == 0 {
		return
	}
	for i := range commands {
		cmd := commands[i]
		s.failLinkedRecord(ctx, &cmd)
		now := time.Now().UTC()
		cmd.Status = model.AgentCommandStatusTimeout
		cmd.ErrorMessage = "agent did not report result before timeout"
		cmd.CompletedAt = &now
		_ = s.cmdRepo.Update(ctx, &cmd)
	}
}

// failLinkedRecord 根据命令类型把关联记录标记为 failed。
// 只对仍然处于 running 状态的记录生效，避免覆盖已完成的结果。
func (s *AgentService) failLinkedRecord(ctx context.Context, cmd *model.AgentCommand) {
	const failureMessage = "Agent 未在超时前回传状态（节点可能已离线或崩溃）"
	switch cmd.Type {
	case model.AgentCommandTypeRunTask:
		var payload struct {
			RecordID uint `json:"recordId"`
		}
		if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil || payload.RecordID == 0 {
			return
		}
		record, err := s.recordRepo.FindByID(ctx, payload.RecordID)
		if err != nil || record == nil || record.Status != model.BackupRecordStatusRunning {
			return
		}
		completedAt := time.Now().UTC()
		record.Status = model.BackupRecordStatusFailed
		record.ErrorMessage = failureMessage
		record.CompletedAt = &completedAt
		record.DurationSeconds = int(completedAt.Sub(record.StartedAt).Seconds())
		_ = s.recordRepo.Update(ctx, record)
	case model.AgentCommandTypeRestoreRecord:
		if s.restoreRepo == nil {
			return
		}
		var payload struct {
			RestoreRecordID uint `json:"restoreRecordId"`
		}
		if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil || payload.RestoreRecordID == 0 {
			return
		}
		restore, err := s.restoreRepo.FindByID(ctx, payload.RestoreRecordID)
		if err != nil || restore == nil || restore.Status != model.RestoreRecordStatusRunning {
			return
		}
		completedAt := time.Now().UTC()
		restore.Status = model.RestoreRecordStatusFailed
		restore.ErrorMessage = failureMessage
		restore.CompletedAt = &completedAt
		restore.DurationSeconds = int(completedAt.Sub(restore.StartedAt).Seconds())
		_ = s.restoreRepo.Update(ctx, restore)
	}
}

// AgentSelfStatus 是 /api/v1/agent/self 端点返回给 Agent 的轻量状态摘要。
type AgentSelfStatus struct {
	ID       uint      `json:"id"`
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"lastSeen"`
}

// SelfStatus 返回 Agent token 所属节点的当前状态，供安装脚本末尾探活。
func (s *AgentService) SelfStatus(ctx context.Context, node *model.Node) (*AgentSelfStatus, error) {
	if node == nil {
		return nil, apperror.Unauthorized("NODE_INVALID_TOKEN", "节点不存在", nil)
	}
	return &AgentSelfStatus{
		ID:       node.ID,
		Name:     node.Name,
		Status:   node.Status,
		LastSeen: node.LastSeen,
	}, nil
}
