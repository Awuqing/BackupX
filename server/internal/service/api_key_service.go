package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// ApiKeyPrefix 所有 API Key 的明文前缀，用于中间件快速识别。
const ApiKeyPrefix = "bax_"

// ApiKeyService 管理 API Key 生命周期。
// 创建时生成 32 字节随机密钥 → 明文一次性返回 → 仅存储 SHA-256 哈希。
// 验证时计算输入的 SHA-256 查表，避免时序攻击和泄漏。
type ApiKeyService struct {
	repo repository.ApiKeyRepository
}

func NewApiKeyService(repo repository.ApiKeyRepository) *ApiKeyService {
	return &ApiKeyService{repo: repo}
}

// ApiKeyCreateInput 创建 API Key 的输入参数。
type ApiKeyCreateInput struct {
	Name     string `json:"name" binding:"required,min=1,max=128"`
	Role     string `json:"role" binding:"required,oneof=admin operator viewer"`
	TTLHours int    `json:"ttlHours"` // 0 表示永不过期
}

// ApiKeyCreateResult 创建后返回给调用者一次。
// PlainKey 只此一次，前端需要告知用户立即保存。
type ApiKeyCreateResult struct {
	ApiKey   ApiKeySummary `json:"apiKey"`
	PlainKey string        `json:"plainKey"`
}

// ApiKeySummary 列表项（无明文）。
type ApiKeySummary struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Role       string     `json:"role"`
	Prefix     string     `json:"prefix"`
	CreatedBy  string     `json:"createdBy"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	Disabled   bool       `json:"disabled"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func (s *ApiKeyService) Create(ctx context.Context, createdBy string, input ApiKeyCreateInput) (*ApiKeyCreateResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, apperror.BadRequest("API_KEY_INVALID", "名称不能为空", nil)
	}
	if !model.IsValidRole(input.Role) {
		return nil, apperror.BadRequest("API_KEY_INVALID", "非法的角色", nil)
	}
	plain, err := generateApiKeyPlain()
	if err != nil {
		return nil, apperror.Internal("API_KEY_GEN_FAILED", "无法生成 API Key", err)
	}
	hash := hashApiKey(plain)
	// Prefix 取前 12 字符供 UI 区分，不泄漏足够信息
	prefix := plain
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	key := &model.ApiKey{
		Name:      name,
		Role:      input.Role,
		KeyHash:   hash,
		Prefix:    prefix,
		CreatedBy: strings.TrimSpace(createdBy),
	}
	if input.TTLHours > 0 {
		expires := time.Now().UTC().Add(time.Duration(input.TTLHours) * time.Hour)
		key.ExpiresAt = &expires
	}
	if err := s.repo.Create(ctx, key); err != nil {
		return nil, apperror.Internal("API_KEY_CREATE_FAILED", "无法创建 API Key", err)
	}
	return &ApiKeyCreateResult{ApiKey: toApiKeySummary(key), PlainKey: plain}, nil
}

func (s *ApiKeyService) List(ctx context.Context) ([]ApiKeySummary, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, apperror.Internal("API_KEY_LIST_FAILED", "无法获取 API Key 列表", err)
	}
	result := make([]ApiKeySummary, 0, len(items))
	for i := range items {
		result = append(result, toApiKeySummary(&items[i]))
	}
	return result, nil
}

// Revoke 撤销指定 API Key（物理删除，保持 db 紧凑）。
func (s *ApiKeyService) Revoke(ctx context.Context, id uint) error {
	key, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperror.Internal("API_KEY_GET_FAILED", "无法获取 API Key", err)
	}
	if key == nil {
		return apperror.New(404, "API_KEY_NOT_FOUND", "API Key 不存在", nil)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return apperror.Internal("API_KEY_DELETE_FAILED", "无法删除 API Key", err)
	}
	return nil
}

// ToggleDisabled 启用/停用 API Key（保留记录便于审计）。
func (s *ApiKeyService) ToggleDisabled(ctx context.Context, id uint, disabled bool) error {
	key, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return apperror.Internal("API_KEY_GET_FAILED", "无法获取 API Key", err)
	}
	if key == nil {
		return apperror.New(404, "API_KEY_NOT_FOUND", "API Key 不存在", nil)
	}
	key.Disabled = disabled
	return s.repo.Update(ctx, key)
}

// Authenticate 实现 http.ApiKeyAuthenticator 接口。
// 返回 (subject, role, error)。subject 形如 "api_key:<id>:<name>"，供审计记录。
func (s *ApiKeyService) Authenticate(ctx context.Context, rawKey string) (string, string, error) {
	rawKey = strings.TrimSpace(rawKey)
	if !strings.HasPrefix(rawKey, ApiKeyPrefix) {
		return "", "", apperror.Unauthorized("AUTH_INVALID_TOKEN", "无效的 API Key 格式", nil)
	}
	hash := hashApiKey(rawKey)
	key, err := s.repo.FindByHash(ctx, hash)
	if err != nil {
		return "", "", apperror.Internal("API_KEY_LOOKUP_FAILED", "无法验证 API Key", err)
	}
	if key == nil {
		return "", "", apperror.Unauthorized("AUTH_INVALID_TOKEN", "API Key 无效", nil)
	}
	if key.Disabled {
		return "", "", apperror.Unauthorized("AUTH_KEY_DISABLED", "API Key 已被停用", nil)
	}
	if key.ExpiresAt != nil && time.Now().UTC().After(*key.ExpiresAt) {
		return "", "", apperror.Unauthorized("AUTH_KEY_EXPIRED", "API Key 已过期", nil)
	}
	// 更新 last_used_at，失败忽略
	_ = s.repo.MarkUsed(ctx, key.ID, time.Now().UTC())
	subject := fmt.Sprintf("api_key:%d:%s", key.ID, key.Name)
	return subject, key.Role, nil
}

func toApiKeySummary(key *model.ApiKey) ApiKeySummary {
	return ApiKeySummary{
		ID:         key.ID,
		Name:       key.Name,
		Role:       key.Role,
		Prefix:     key.Prefix,
		CreatedBy:  key.CreatedBy,
		LastUsedAt: key.LastUsedAt,
		ExpiresAt:  key.ExpiresAt,
		Disabled:   key.Disabled,
		CreatedAt:  key.CreatedAt,
	}
}

// generateApiKeyPlain 生成 bax_<32hex> 格式的密钥。
func generateApiKeyPlain() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return ApiKeyPrefix + hex.EncodeToString(buf), nil
}

// hashApiKey 对明文进行 SHA-256 哈希。不加盐：明文本身已是 192 位随机值，无字典攻击风险。
func hashApiKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
