package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// InstallTokenService 负责一次性安装令牌的创建/消费/校验。
type InstallTokenService struct {
	repo     repository.AgentInstallTokenRepository
	nodeRepo repository.NodeRepository
}

func NewInstallTokenService(repo repository.AgentInstallTokenRepository, nodeRepo repository.NodeRepository) *InstallTokenService {
	return &InstallTokenService{repo: repo, nodeRepo: nodeRepo}
}

// InstallTokenInput 生成一次性安装令牌的输入。
type InstallTokenInput struct {
	NodeID       uint
	Mode         string
	Arch         string
	AgentVersion string
	DownloadSrc  string
	TTLSeconds   int
	CreatedByID  uint
}

// InstallTokenOutput 生成结果。
type InstallTokenOutput struct {
	Token     string
	ExpiresAt time.Time
	Node      *model.Node
	Record    *model.AgentInstallToken
}

// ConsumedInstallToken 消费成功后返回给 handler 的组合体。
type ConsumedInstallToken struct {
	Record *model.AgentInstallToken
	Node   *model.Node
}

// 校验与限流常量。
const (
	InstallTokenMinTTL     = 300   // 5 分钟
	InstallTokenMaxTTL     = 86400 // 24 小时
	InstallTokenRateWindow = 60 * time.Second
	InstallTokenRatePerWin = 5
)

var (
	validInstallModes   = map[string]bool{model.InstallModeSystemd: true, model.InstallModeDocker: true, model.InstallModeForeground: true}
	validInstallArches  = map[string]bool{model.InstallArchAmd64: true, model.InstallArchArm64: true, model.InstallArchAuto: true}
	validInstallSources = map[string]bool{model.InstallSourceGitHub: true, model.InstallSourceGhproxy: true}
)

// Create 生成一次性安装令牌。
func (s *InstallTokenService) Create(ctx context.Context, in InstallTokenInput) (*InstallTokenOutput, error) {
	if err := s.validate(in); err != nil {
		return nil, err
	}
	node, err := s.nodeRepo.FindByID(ctx, in.NodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, apperror.New(404, "NODE_NOT_FOUND", "节点不存在", nil)
	}

	since := time.Now().UTC().Add(-InstallTokenRateWindow)
	count, err := s.repo.CountCreatedSince(ctx, in.NodeID, since)
	if err != nil {
		return nil, err
	}
	if count >= InstallTokenRatePerWin {
		return nil, apperror.TooManyRequests("INSTALL_TOKEN_RATE_LIMITED",
			fmt.Sprintf("每 %d 秒最多生成 %d 次", int(InstallTokenRateWindow.Seconds()), InstallTokenRatePerWin), nil)
	}

	token, err := generateInstallToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(time.Duration(in.TTLSeconds) * time.Second)
	record := &model.AgentInstallToken{
		Token:       token,
		NodeID:      in.NodeID,
		Mode:        in.Mode,
		Arch:        in.Arch,
		AgentVer:    in.AgentVersion,
		DownloadSrc: in.DownloadSrc,
		ExpiresAt:   expiresAt,
		CreatedByID: in.CreatedByID,
	}
	if err := s.repo.Create(ctx, record); err != nil {
		return nil, err
	}
	return &InstallTokenOutput{Token: token, ExpiresAt: expiresAt, Node: node, Record: record}, nil
}

// Consume 原子消费令牌。未命中/已过期/已消费均返回 (nil, nil)。
func (s *InstallTokenService) Consume(ctx context.Context, token string) (*ConsumedInstallToken, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	record, err := s.repo.ConsumeByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	node, err := s.nodeRepo.FindByID(ctx, record.NodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, apperror.New(404, "NODE_NOT_FOUND", "节点已被删除", nil)
	}
	return &ConsumedInstallToken{Record: record, Node: node}, nil
}

// Peek 只读查询（不消费），供 compose 端点预检 Mode。
func (s *InstallTokenService) Peek(ctx context.Context, token string) (*model.AgentInstallToken, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	return s.repo.FindByToken(ctx, token)
}

// StartGC 启动后台 GC，按 interval 扫描并删 ExpiresAt < now-7d 的记录。
func (s *InstallTokenService) StartGC(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.repo.DeleteExpiredBefore(ctx, time.Now().UTC().Add(-7*24*time.Hour))
			}
		}
	}()
}

func (s *InstallTokenService) validate(in InstallTokenInput) error {
	if in.NodeID == 0 {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID", "nodeId 必填", nil)
	}
	if !validInstallModes[in.Mode] {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID", "mode 非法", nil)
	}
	if !validInstallArches[in.Arch] {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID", "arch 非法", nil)
	}
	if !validInstallSources[in.DownloadSrc] {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID", "downloadSrc 非法", nil)
	}
	if strings.TrimSpace(in.AgentVersion) == "" {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID", "agentVersion 必填", nil)
	}
	if in.TTLSeconds < InstallTokenMinTTL || in.TTLSeconds > InstallTokenMaxTTL {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID",
			fmt.Sprintf("ttlSeconds 需在 %d-%d", InstallTokenMinTTL, InstallTokenMaxTTL), nil)
	}
	return nil
}

func generateInstallToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
