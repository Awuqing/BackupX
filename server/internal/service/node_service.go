package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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

// NodeService manages the cluster nodes.
type NodeService struct {
	repo repository.NodeRepository
}

func NewNodeService(repo repository.NodeRepository) *NodeService {
	return &NodeService{repo: repo}
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
		existing.OS = runtime.GOOS
		existing.Arch = runtime.GOARCH
		return s.repo.Update(ctx, existing)
	}
	hostname, _ := os.Hostname()
	token, _ := generateToken()
	node := &model.Node{
		Name:     "本机 (Local)",
		Hostname: hostname,
		Token:    token,
		Status:   model.NodeStatusOnline,
		IsLocal:  true,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		LastSeen: time.Now().UTC(),
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
		return nil, apperror.BadRequest("NODE_REMOTE_FS_NOT_SUPPORTED", "远程节点的目录浏览需要 Agent 在线连接（即将支持）", nil)
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

// Heartbeat updates the node status when an agent reports in.
func (s *NodeService) Heartbeat(ctx context.Context, token string, hostname string, ip string, agentVer string) error {
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
	node.OS = runtime.GOOS
	node.Arch = runtime.GOARCH
	node.LastSeen = time.Now().UTC()
	return s.repo.Update(ctx, node)
}

// DirEntry represents a file or directory in a node's file system.
type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
