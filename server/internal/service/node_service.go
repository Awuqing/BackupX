package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// NodeSummary is the API response for node listings.
type NodeSummary struct {
	ID           uint      `json:"id"`
	Name         string    `json:"name"`
	Hostname     string    `json:"hostname"`
	IPAddress    string    `json:"ipAddress"`
	Status       string    `json:"status"`
	IsLocal      bool      `json:"isLocal"`
	OS           string    `json:"os"`
	Arch         string    `json:"arch"`
	AgentVersion string    `json:"agentVersion"`
	LastSeen     time.Time `json:"lastSeen"`
	CreatedAt    time.Time `json:"createdAt"`
}

// NodeCreateInput is the input for creating a new remote node.
type NodeCreateInput struct {
	Name string `json:"name" binding:"required"`
}

// NodeUpdateInput 是编辑节点的输入。
type NodeUpdateInput struct {
	Name string `json:"name" binding:"required"`
}

// NodeService manages the cluster nodes.
type NodeService struct {
	repo      repository.NodeRepository
	taskRepo  repository.BackupTaskRepository
	agentRPC  NodeAgentRPC
	version   string
}

// NodeAgentRPC 抽象 Agent 远程调用能力（避免 service 内循环依赖）。
// 由 AgentService 实现；当 Agent 未启用时可不注入，远程目录浏览返回提示。
type NodeAgentRPC interface {
	EnqueueCommand(ctx context.Context, nodeID uint, cmdType string, payload any) (uint, error)
	WaitForCommandResult(ctx context.Context, cmdID uint, timeout time.Duration) (*model.AgentCommand, error)
}

func NewNodeService(repo repository.NodeRepository, version string) *NodeService {
	return &NodeService{repo: repo, version: version}
}

// SetTaskRepository 注入任务仓储以支持删除前引用检查。可选注入，便于测试。
func (s *NodeService) SetTaskRepository(taskRepo repository.BackupTaskRepository) {
	s.taskRepo = taskRepo
}

// SetAgentRPC 注入 Agent RPC 能力，启用远程目录浏览。
func (s *NodeService) SetAgentRPC(rpc NodeAgentRPC) {
	s.agentRPC = rpc
}

// EnsureLocalNode creates the default "local" node if it does not exist.
func (s *NodeService) EnsureLocalNode(ctx context.Context) error {
	existing, err := s.repo.FindLocal(ctx)
	if err != nil {
		return err
	}
	if existing != nil {
		existing.Status = model.NodeStatusOnline
		existing.LastSeen = time.Now().UTC()
		hostname, _ := os.Hostname()
		existing.Hostname = hostname
		existing.IPAddress = detectLocalIP()
		existing.AgentVer = s.version
		existing.OS = runtime.GOOS
		existing.Arch = runtime.GOARCH
		return s.repo.Update(ctx, existing)
	}
	hostname, _ := os.Hostname()
	token, _ := generateToken()
	node := &model.Node{
		Name:      "本机 (Local)",
		Hostname:  hostname,
		IPAddress: detectLocalIP(),
		Token:     token,
		Status:    model.NodeStatusOnline,
		IsLocal:   true,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		AgentVer:  s.version,
		LastSeen:  time.Now().UTC(),
	}
	return s.repo.Create(ctx, node)
}

func (s *NodeService) List(ctx context.Context) ([]NodeSummary, error) {
	nodes, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]NodeSummary, len(nodes))
	for i, n := range nodes {
		result[i] = NodeSummary{
			ID:           n.ID,
			Name:         n.Name,
			Hostname:     n.Hostname,
			IPAddress:    n.IPAddress,
			Status:       n.Status,
			IsLocal:      n.IsLocal,
			OS:           n.OS,
			Arch:         n.Arch,
			AgentVersion: n.AgentVer,
			LastSeen:     n.LastSeen,
			CreatedAt:    n.CreatedAt,
		}
	}
	return result, nil
}

func (s *NodeService) Get(ctx context.Context, id uint) (*NodeSummary, error) {
	node, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, apperror.New(http.StatusNotFound, "NODE_NOT_FOUND", "节点不存在", nil)
	}
	return &NodeSummary{
		ID:           node.ID,
		Name:         node.Name,
		Hostname:     node.Hostname,
		IPAddress:    node.IPAddress,
		Status:       node.Status,
		IsLocal:      node.IsLocal,
		OS:           node.OS,
		Arch:         node.Arch,
		AgentVersion: node.AgentVer,
		LastSeen:     node.LastSeen,
		CreatedAt:    node.CreatedAt,
	}, nil
}

// Create registers a new remote node and returns its authentication token.
func (s *NodeService) Create(ctx context.Context, input NodeCreateInput) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	node := &model.Node{
		Name:     input.Name,
		Token:    token,
		Status:   model.NodeStatusOffline,
		IsLocal:  false,
		LastSeen: time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, node); err != nil {
		return "", err
	}
	return token, nil
}

func (s *NodeService) Delete(ctx context.Context, id uint) error {
	node, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if node == nil {
		return apperror.New(http.StatusNotFound, "NODE_NOT_FOUND", "节点不存在", nil)
	}
	if node.IsLocal {
		return apperror.BadRequest("NODE_DELETE_LOCAL", "无法删除本机节点", nil)
	}
	// 删除前检查是否有关联备份任务，避免孤立任务
	if s.taskRepo != nil {
		count, err := s.taskRepo.CountByNodeID(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return apperror.BadRequest(
				"NODE_HAS_TASKS",
				fmt.Sprintf("无法删除：该节点上还有 %d 个备份任务，请先删除或迁移", count),
				nil,
			)
		}
	}
	return s.repo.Delete(ctx, id)
}

// ListDirectory lists the contents of a directory on the local node.
func (s *NodeService) ListDirectory(ctx context.Context, nodeID uint, path string) ([]DirEntry, error) {
	node, err := s.repo.FindByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, apperror.New(http.StatusNotFound, "NODE_NOT_FOUND", "节点不存在", nil)
	}
	if !node.IsLocal {
		// 远程节点：通过 Agent 命令队列做同步 RPC
		return s.remoteListDirectory(ctx, node, path)
	}

	cleanPath := filepath.Clean(path)
	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		return nil, apperror.BadRequest("NODE_FS_READ_ERROR", fmt.Sprintf("无法读取目录: %s", err.Error()), err)
	}

	result := make([]DirEntry, 0, len(entries))
	for _, entry := range entries {
		info, _ := entry.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		result = append(result, DirEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(cleanPath, entry.Name()),
			IsDir: entry.IsDir(),
			Size:  size,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// OfflineThreshold 节点被判定为离线的心跳超时阈值。
// Agent 默认 15s 心跳一次；45s 未见视为离线，预留 3 次重试空间。
const OfflineThreshold = 45 * time.Second

// StartOfflineMonitor 启动后台 goroutine，定期把超时未心跳的节点标记为离线。
// 传入的 ctx 被取消后退出。
func (s *NodeService) StartOfflineMonitor(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				threshold := time.Now().UTC().Add(-OfflineThreshold)
				_, _ = s.repo.MarkStaleOffline(ctx, threshold)
			}
		}
	}()
}

// Heartbeat updates the node status when an agent reports in.
func (s *NodeService) Heartbeat(ctx context.Context, token string, hostname string, ip string, agentVer string, osName string, archName string) error {
	node, err := s.repo.FindByToken(ctx, token)
	if err != nil {
		return err
	}
	if node == nil {
		return apperror.Unauthorized("NODE_INVALID_TOKEN", "无效的节点认证令牌", nil)
	}
	node.Status = model.NodeStatusOnline
	node.Hostname = hostname
	node.IPAddress = ip
	node.AgentVer = agentVer
	if strings.TrimSpace(osName) != "" {
		node.OS = osName
	} else {
		node.OS = runtime.GOOS
	}
	if strings.TrimSpace(archName) != "" {
		node.Arch = archName
	} else {
		node.Arch = runtime.GOARCH
	}
	node.LastSeen = time.Now().UTC()
	return s.repo.Update(ctx, node)
}

// Update 编辑节点名称。
func (s *NodeService) Update(ctx context.Context, id uint, input NodeUpdateInput) (*NodeSummary, error) {
	node, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, apperror.New(http.StatusNotFound, "NODE_NOT_FOUND", "节点不存在", nil)
	}
	node.Name = strings.TrimSpace(input.Name)
	if err := s.repo.Update(ctx, node); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// DirEntry represents a file or directory in a node's file system.
type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// remoteListDirectory 通过命令队列下发 list_dir 给 Agent 并同步等待结果。
// Agent 必须在线，且响应需在 15s 内返回，否则返回超时错误。
func (s *NodeService) remoteListDirectory(ctx context.Context, node *model.Node, path string) ([]DirEntry, error) {
	if s.agentRPC == nil {
		return nil, apperror.BadRequest("NODE_REMOTE_FS_NOT_SUPPORTED", "远程目录浏览未启用，需要 Master 启用 Agent 服务", nil)
	}
	if node.Status != model.NodeStatusOnline {
		return nil, apperror.BadRequest("NODE_OFFLINE", "节点当前不在线，无法浏览其目录", nil)
	}
	if strings.TrimSpace(path) == "" {
		path = "/"
	}
	cmdID, err := s.agentRPC.EnqueueCommand(ctx, node.ID, model.AgentCommandTypeListDir, map[string]any{"path": path})
	if err != nil {
		return nil, apperror.Internal("AGENT_COMMAND_ENQUEUE_FAILED", "下发目录浏览命令失败", err)
	}
	cmd, err := s.agentRPC.WaitForCommandResult(ctx, cmdID, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if cmd.Status != model.AgentCommandStatusSucceeded {
		msg := cmd.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("command status: %s", cmd.Status)
		}
		return nil, apperror.BadRequest("NODE_FS_READ_ERROR", fmt.Sprintf("远程目录浏览失败: %s", msg), nil)
	}
	var result struct {
		Entries []DirEntry `json:"entries"`
	}
	if err := json.Unmarshal([]byte(cmd.Result), &result); err != nil {
		return nil, apperror.Internal("AGENT_RESULT_INVALID", "Agent 返回结果格式错误", err)
	}
	return result.Entries, nil
}

// detectLocalIP 获取本机第一个非回环 IPv4 地址。
func detectLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return ""
}

// NodeCreateResult 批量创建结果。注意：不暴露 agent token，token 获取走 install-token 流程。
type NodeCreateResult struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// BatchCreate 批量创建远程节点。
// 校验：1-50 项、每项 1-128 字符、批次内去重、与已有节点名去重。
// 返回 NodeCreateResult 列表（不含 token，调用方应再调 install-tokens 接口）。
func (s *NodeService) BatchCreate(ctx context.Context, names []string) ([]NodeCreateResult, error) {
	cleaned, err := validateBatchNames(names)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	existingSet := make(map[string]bool, len(existing))
	for _, n := range existing {
		existingSet[n.Name] = true
	}
	for _, name := range cleaned {
		if existingSet[name] {
			return nil, apperror.BadRequest("NODE_DUPLICATE_NAME",
				fmt.Sprintf("节点名「%s」已存在", name), nil)
		}
	}

	// 预先构造所有 Node，token 生成在事务外完成（纯内存操作，失败不会影响 DB 状态）
	nodes := make([]*model.Node, 0, len(cleaned))
	now := time.Now().UTC()
	for _, name := range cleaned {
		tok, err := generateToken()
		if err != nil {
			return nil, fmt.Errorf("generate token: %w", err)
		}
		nodes = append(nodes, &model.Node{
			Name:     name,
			Token:    tok,
			Status:   model.NodeStatusOffline,
			IsLocal:  false,
			LastSeen: now,
		})
	}
	// 事务内批量创建：任一失败整体回滚
	if err := s.repo.BatchCreate(ctx, nodes); err != nil {
		return nil, err
	}
	results := make([]NodeCreateResult, 0, len(nodes))
	for _, n := range nodes {
		results = append(results, NodeCreateResult{ID: n.ID, Name: n.Name})
	}
	return results, nil
}

// RotateToken 轮换指定节点的 agent token。
// 旧 token 复制到 prev_token，24h 内新旧 token 均可认证。
func (s *NodeService) RotateToken(ctx context.Context, id uint) (string, error) {
	node, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if node == nil {
		return "", apperror.New(http.StatusNotFound, "NODE_NOT_FOUND", "节点不存在", nil)
	}
	if node.IsLocal {
		return "", apperror.BadRequest("NODE_ROTATE_LOCAL", "本机节点无需轮换 Token", nil)
	}
	newTok, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	expires := time.Now().UTC().Add(24 * time.Hour)
	node.PrevToken = node.Token
	node.PrevTokenExpires = &expires
	node.Token = newTok
	if err := s.repo.Update(ctx, node); err != nil {
		return "", err
	}
	return newTok, nil
}

// validateBatchNames 校验并去重批次内名称（空白行忽略）。
func validateBatchNames(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, apperror.BadRequest("NODE_BATCH_EMPTY", "节点名列表不能为空", nil)
	}
	if len(names) > 50 {
		return nil, apperror.BadRequest("NODE_BATCH_TOO_MANY", "单次最多创建 50 个节点", nil)
	}
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if len(name) > 128 {
			return nil, apperror.BadRequest("NODE_NAME_TOO_LONG",
				fmt.Sprintf("节点名「%s」超过 128 字符", name), nil)
		}
		if seen[name] {
			return nil, apperror.BadRequest("NODE_DUPLICATE_NAME",
				fmt.Sprintf("批次内重复节点名「%s」", name), nil)
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, apperror.BadRequest("NODE_BATCH_EMPTY", "去除空白后列表为空", nil)
	}
	return out, nil
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
