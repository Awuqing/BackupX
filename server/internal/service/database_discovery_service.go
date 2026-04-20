package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/backup"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

type DatabaseDiscoverInput struct {
	Type     string `json:"type" binding:"required,oneof=mysql postgresql"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required,min=1"`
	User     string `json:"user" binding:"required"`
	Password string `json:"password" binding:"required"`
	// NodeID 执行发现的节点。0 或本机 → Master 本地执行；
	// 远程节点 → 通过 Agent RPC 下发 discover_db 命令，目标主机在该节点视角解析。
	NodeID uint `json:"nodeId"`
}

type DatabaseDiscoverResult struct {
	Databases []string `json:"databases"`
}

type DatabaseDiscoveryService struct {
	executor backup.CommandExecutor
	nodeRepo repository.NodeRepository
	agentRPC DatabaseDiscoveryAgentRPC
}

// DatabaseDiscoveryAgentRPC 封装 AgentService 的同步 RPC 能力以避免循环依赖。
type DatabaseDiscoveryAgentRPC interface {
	EnqueueCommand(ctx context.Context, nodeID uint, cmdType string, payload any) (uint, error)
	WaitForCommandResult(ctx context.Context, cmdID uint, timeout time.Duration) (*model.AgentCommand, error)
}

func NewDatabaseDiscoveryService(executor backup.CommandExecutor) *DatabaseDiscoveryService {
	return &DatabaseDiscoveryService{executor: executor}
}

// SetClusterDependencies 注入集群依赖，启用远程节点发现。
// 可选注入：未注入时仅支持在 Master 本地发现。
func (s *DatabaseDiscoveryService) SetClusterDependencies(nodeRepo repository.NodeRepository, rpc DatabaseDiscoveryAgentRPC) {
	s.nodeRepo = nodeRepo
	s.agentRPC = rpc
}

func (s *DatabaseDiscoveryService) Discover(ctx context.Context, input DatabaseDiscoverInput) (*DatabaseDiscoverResult, error) {
	dbType := strings.TrimSpace(strings.ToLower(input.Type))
	if dbType != "mysql" && dbType != "postgresql" {
		return nil, apperror.BadRequest("DATABASE_DISCOVER_INVALID_TYPE", "不支持的数据库类型", nil)
	}
	// 远程节点路由
	if s.shouldRouteToAgent(ctx, input.NodeID) {
		return s.discoverViaAgent(ctx, input)
	}
	// 本地执行
	databases, err := backup.DiscoverDatabases(ctx, s.executor, backup.DiscoverRequest{
		Type:     dbType,
		Host:     input.Host,
		Port:     input.Port,
		User:     input.User,
		Password: input.Password,
	})
	if err != nil {
		// 统一映射为 BadRequest，便于前端显示
		return nil, apperror.BadRequest("DATABASE_DISCOVER_FAILED", sanitizeMessage(err.Error()), err)
	}
	return &DatabaseDiscoverResult{Databases: databases}, nil
}

// shouldRouteToAgent 判断是否应路由到远程 Agent 执行发现。
// NodeID=0、未注入集群依赖、或节点为本机时返回 false。
func (s *DatabaseDiscoveryService) shouldRouteToAgent(ctx context.Context, nodeID uint) bool {
	if nodeID == 0 || s.nodeRepo == nil || s.agentRPC == nil {
		return false
	}
	node, err := s.nodeRepo.FindByID(ctx, nodeID)
	if err != nil || node == nil || node.IsLocal {
		return false
	}
	return true
}

// discoverViaAgent 下发 discover_db 命令到 Agent 并同步等待结果。
// Agent 必须在线；命令 15s 内未返回视为超时。
func (s *DatabaseDiscoveryService) discoverViaAgent(ctx context.Context, input DatabaseDiscoverInput) (*DatabaseDiscoverResult, error) {
	node, err := s.nodeRepo.FindByID(ctx, input.NodeID)
	if err != nil {
		return nil, apperror.Internal("DATABASE_DISCOVER_NODE_LOOKUP_FAILED", "无法读取节点", err)
	}
	if node == nil {
		return nil, apperror.BadRequest("DATABASE_DISCOVER_NODE_NOT_FOUND", "指定的节点不存在", nil)
	}
	if node.Status != model.NodeStatusOnline {
		return nil, apperror.BadRequest("NODE_OFFLINE", fmt.Sprintf("节点 %s 当前离线，无法执行数据库发现", node.Name), nil)
	}
	cmdID, err := s.agentRPC.EnqueueCommand(ctx, node.ID, model.AgentCommandTypeDiscoverDB, map[string]any{
		"type":     strings.ToLower(input.Type),
		"host":     input.Host,
		"port":     input.Port,
		"user":     input.User,
		"password": input.Password,
	})
	if err != nil {
		return nil, apperror.Internal("AGENT_COMMAND_ENQUEUE_FAILED", "无法下发数据库发现命令", err)
	}
	cmd, err := s.agentRPC.WaitForCommandResult(ctx, cmdID, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if cmd.Status != model.AgentCommandStatusSucceeded {
		msg := strings.TrimSpace(cmd.ErrorMessage)
		if msg == "" {
			msg = fmt.Sprintf("命令状态: %s", cmd.Status)
		}
		return nil, apperror.BadRequest("DATABASE_DISCOVER_FAILED", sanitizeMessage(msg), nil)
	}
	var result struct {
		Databases []string `json:"databases"`
	}
	if err := json.Unmarshal([]byte(cmd.Result), &result); err != nil {
		return nil, apperror.Internal("AGENT_RESULT_INVALID", "Agent 返回结果格式错误", err)
	}
	return &DatabaseDiscoverResult{Databases: result.Databases}, nil
}
