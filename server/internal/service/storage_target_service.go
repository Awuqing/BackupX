package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
	goauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

type StorageTargetUpsertInput struct {
	Name        string         `json:"name" binding:"required,min=1,max=128"`
	Type        string         `json:"type" binding:"required,min=1"`
	Description string         `json:"description" binding:"max=255"`
	Enabled     bool           `json:"enabled"`
	Config      map[string]any `json:"config" binding:"required"`
	// QuotaBytes 软限额（字节），0 = 不限制。
	QuotaBytes int64 `json:"quotaBytes"`
}

type StorageTargetTestInput struct {
	TargetID *uint                    `json:"targetId"`
	Payload  StorageTargetUpsertInput `json:"payload"`
}

type GoogleDriveAuthStartInput struct {
	TargetID     *uint  `json:"targetId"`
	Name         string `json:"name" binding:"required,min=1,max=128"`
	Description  string `json:"description" binding:"max=255"`
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"clientId" binding:"required"`
	ClientSecret string `json:"clientSecret" binding:"required"`
	FolderID     string `json:"folderId"`
}

type GoogleDriveAuthCompleteInput struct {
	State string `json:"state" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

type StorageTargetSummary struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Description     string     `json:"description"`
	Enabled         bool       `json:"enabled"`
	Starred         bool       `json:"starred"`
	ConfigVersion   int        `json:"configVersion"`
	LastTestedAt    *time.Time `json:"lastTestedAt"`
	LastTestStatus  string     `json:"lastTestStatus"`
	LastTestMessage string     `json:"lastTestMessage"`
	QuotaBytes      int64      `json:"quotaBytes"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type StorageTargetDetail struct {
	StorageTargetSummary
	Config       map[string]any `json:"config"`
	MaskedFields []string       `json:"maskedFields,omitempty"`
}

type GoogleDriveAuthStartResult struct {
	AuthorizationURL string    `json:"authorizationUrl"`
	State            string    `json:"state"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type googleDriveOAuthDraft struct {
	TargetID     *uint  `json:"targetId"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	FolderID     string `json:"folderId"`
	RedirectURI  string `json:"redirectUri"`
}

type StorageTargetService struct {
	targets       repository.StorageTargetRepository
	oauthSessions repository.OAuthSessionRepository
	backupTasks   repository.BackupTaskRepository
	records       repository.BackupRecordRepository
	registry      *storage.Registry
	cipher        *codec.ConfigCipher
}

func NewStorageTargetService(
	targets repository.StorageTargetRepository,
	oauthSessions repository.OAuthSessionRepository,
	registry *storage.Registry,
	cipher *codec.ConfigCipher,
) *StorageTargetService {
	return &StorageTargetService{targets: targets, oauthSessions: oauthSessions, registry: registry, cipher: cipher}
}

func (s *StorageTargetService) SetBackupTaskRepository(tasks repository.BackupTaskRepository) {
	s.backupTasks = tasks
}

func (s *StorageTargetService) SetBackupRecordRepository(records repository.BackupRecordRepository) {
	s.records = records
}

func (s *StorageTargetService) List(ctx context.Context) ([]StorageTargetSummary, error) {
	items, err := s.targets.List(ctx)
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_LIST_FAILED", "无法获取存储目标列表", err)
	}
	result := make([]StorageTargetSummary, 0, len(items))
	for _, item := range items {
		result = append(result, toStorageTargetSummary(&item))
	}
	return result, nil
}

func (s *StorageTargetService) Get(ctx context.Context, id uint) (*StorageTargetDetail, error) {
	item, err := s.targets.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_GET_FAILED", "无法获取存储目标详情", err)
	}
	if item == nil {
		return nil, apperror.New(http.StatusNotFound, "STORAGE_TARGET_NOT_FOUND", "存储目标不存在", fmt.Errorf("storage target %d not found", id))
	}
	configMap, err := s.decryptTargetConfig(item)
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_DECRYPT_FAILED", "无法解密存储目标配置", err)
	}
	sensitiveFields := s.registry.SensitiveFields(storage.ParseProviderType(item.Type))
	return &StorageTargetDetail{StorageTargetSummary: toStorageTargetSummary(item), Config: codec.MaskConfig(configMap, sensitiveFields), MaskedFields: sensitiveFields}, nil
}

func (s *StorageTargetService) Create(ctx context.Context, input StorageTargetUpsertInput) (*StorageTargetDetail, error) {
	if err := s.validateType(input.Type); err != nil {
		return nil, err
	}
	existing, err := s.targets.FindByName(ctx, strings.TrimSpace(input.Name))
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_LOOKUP_FAILED", "无法检查存储目标名称", err)
	}
	if existing != nil {
		return nil, apperror.Conflict("STORAGE_TARGET_NAME_EXISTS", "存储目标名称已存在", nil)
	}
	item, err := s.buildStorageTarget(ctx, nil, input)
	if err != nil {
		return nil, err
	}
	if err := s.targets.Create(ctx, item); err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_CREATE_FAILED", "无法创建存储目标", err)
	}
	return s.Get(ctx, item.ID)
}

func (s *StorageTargetService) Update(ctx context.Context, id uint, input StorageTargetUpsertInput) (*StorageTargetDetail, error) {
	if err := s.validateType(input.Type); err != nil {
		return nil, err
	}
	existing, err := s.targets.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_GET_FAILED", "无法获取存储目标详情", err)
	}
	if existing == nil {
		return nil, apperror.New(http.StatusNotFound, "STORAGE_TARGET_NOT_FOUND", "存储目标不存在", fmt.Errorf("storage target %d not found", id))
	}
	if sameName, err := s.targets.FindByName(ctx, strings.TrimSpace(input.Name)); err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_LOOKUP_FAILED", "无法检查存储目标名称", err)
	} else if sameName != nil && sameName.ID != existing.ID {
		return nil, apperror.Conflict("STORAGE_TARGET_NAME_EXISTS", "存储目标名称已存在", nil)
	}
	item, err := s.buildStorageTarget(ctx, existing, input)
	if err != nil {
		return nil, err
	}
	item.ID = existing.ID
	item.CreatedAt = existing.CreatedAt
	if err := s.targets.Update(ctx, item); err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_UPDATE_FAILED", "无法更新存储目标", err)
	}
	return s.Get(ctx, item.ID)
}

func (s *StorageTargetService) Delete(ctx context.Context, id uint) error {
	existing, err := s.targets.FindByID(ctx, id)
	if err != nil {
		return apperror.Internal("STORAGE_TARGET_GET_FAILED", "无法获取存储目标详情", err)
	}
	if existing == nil {
		return apperror.New(http.StatusNotFound, "STORAGE_TARGET_NOT_FOUND", "存储目标不存在", fmt.Errorf("storage target %d not found", id))
	}
	if s.backupTasks != nil {
		count, countErr := s.backupTasks.CountByStorageTargetID(ctx, id)
		if countErr != nil {
			return apperror.Internal("STORAGE_TARGET_REF_CHECK_FAILED", "无法检查存储目标引用关系", countErr)
		}
		if count > 0 {
			return apperror.Conflict("STORAGE_TARGET_IN_USE", "当前存储目标已被备份任务引用，无法删除", nil)
		}
	}
	if err := s.targets.Delete(ctx, id); err != nil {
		return apperror.Internal("STORAGE_TARGET_DELETE_FAILED", "无法删除存储目标", err)
	}
	return nil
}

func (s *StorageTargetService) ToggleStar(ctx context.Context, id uint) (*StorageTargetSummary, error) {
	item, err := s.targets.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_GET_FAILED", "无法获取存储目标详情", err)
	}
	if item == nil {
		return nil, apperror.New(http.StatusNotFound, "STORAGE_TARGET_NOT_FOUND", "存储目标不存在", fmt.Errorf("storage target %d not found", id))
	}
	item.Starred = !item.Starred
	if err := s.targets.Update(ctx, item); err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_UPDATE_FAILED", "无法更新存储目标收藏状态", err)
	}
	summary := toStorageTargetSummary(item)
	return &summary, nil
}

func (s *StorageTargetService) TestConnection(ctx context.Context, input StorageTargetTestInput) error {
	item, err := s.buildStorageTargetForTest(ctx, input)
	if err != nil {
		return err
	}
	configMap, err := s.decryptTargetConfig(item)
	if err != nil {
		return apperror.Internal("STORAGE_TARGET_DECRYPT_FAILED", "无法解密存储目标配置", err)
	}
	provider, err := s.registry.Create(ctx, storage.ParseProviderType(item.Type), configMap)
	if err != nil {
		return apperror.BadRequest("STORAGE_TARGET_INVALID_CONFIG", sanitizeMessage(err.Error()), err)
	}
	testErr := provider.TestConnection(ctx)
	now := time.Now().UTC()
	item.LastTestedAt = &now
	if testErr != nil {
		item.LastTestStatus = "failed"
		item.LastTestMessage = sanitizeMessage(testErr.Error())
	} else {
		item.LastTestStatus = "success"
		item.LastTestMessage = "连接成功"
	}
	if item.ID != 0 {
		_ = s.targets.Update(ctx, item)
	}
	if testErr != nil {
		return apperror.BadRequest("STORAGE_TARGET_TEST_FAILED", sanitizeMessage(testErr.Error()), testErr)
	}
	return nil
}

// StartHealthMonitor 启动后台存储目标健康扫描。
// 周期性对启用的存储目标跑 TestConnection（非阻塞），并在"从成功转失败"时派发 storage_unhealthy 事件。
// interval 建议 5m；dispatcher 为 nil 时仅更新 LastTestStatus 不告警。
func (s *StorageTargetService) StartHealthMonitor(ctx context.Context, dispatcher EventDispatcher, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	// notified 跟踪已告警的目标，避免每轮重复
	notified := map[uint]bool{}
	capacityNotified := map[uint]bool{}
	var mu sync.Mutex
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runHealthCheckOnce(ctx, dispatcher, &mu, notified)
				s.runCapacityCheckOnce(ctx, dispatcher, &mu, capacityNotified)
			}
		}
	}()
}

// StorageCapacityWarningThreshold 存储使用率告警阈值（85%）。
// 超过此值视为容量预警，派发 storage_capacity_warning 事件。
// 做成常量而非配置：企业运维场景下 85% 是业界通用预警线，无需用户调整。
const StorageCapacityWarningThreshold = 0.85

// runCapacityCheckOnce 扫描所有支持 StorageAbout 接口的启用存储目标，
// 使用率超过阈值时派发 storage_capacity_warning 事件（避免重复派发）。
// 降到阈值以下（例如清理/扩容后）自动清除记忆。
func (s *StorageTargetService) runCapacityCheckOnce(ctx context.Context, dispatcher EventDispatcher, mu *sync.Mutex, notified map[uint]bool) {
	if dispatcher == nil {
		return
	}
	targets, err := s.targets.List(ctx)
	if err != nil {
		return
	}
	for i := range targets {
		target := targets[i]
		if !target.Enabled {
			continue
		}
		configMap := map[string]any{}
		if err := s.cipher.DecryptJSON(target.ConfigCiphertext, &configMap); err != nil {
			continue
		}
		provider, err := s.registry.Create(ctx, storage.ParseProviderType(target.Type), configMap)
		if err != nil {
			continue
		}
		about, ok := provider.(storage.StorageAbout)
		if !ok {
			continue // 该后端不支持容量查询（如 S3 / FTP 等），跳过
		}
		info, err := about.About(ctx)
		if err != nil || info == nil || info.Total == nil || info.Used == nil || *info.Total == 0 {
			continue
		}
		usage := float64(*info.Used) / float64(*info.Total)
		mu.Lock()
		alreadyNotified := notified[target.ID]
		if usage >= StorageCapacityWarningThreshold {
			if !alreadyNotified {
				notified[target.ID] = true
				mu.Unlock()
				s.dispatchCapacityWarning(ctx, dispatcher, &target, info, usage)
				continue
			}
		} else {
			delete(notified, target.ID) // 容量回落后允许下次再告警
		}
		mu.Unlock()
	}
}

func (s *StorageTargetService) dispatchCapacityWarning(ctx context.Context, dispatcher EventDispatcher, target *model.StorageTarget, info *storage.StorageUsageInfo, usage float64) {
	title := "BackupX 存储容量预警"
	usedGB := float64(*info.Used) / (1 << 30)
	totalGB := float64(*info.Total) / (1 << 30)
	body := fmt.Sprintf("存储目标：%s (类型: %s)\n使用率：%.1f%%\n已用：%.2f GB / 总量：%.2f GB\n建议清理旧备份或扩容。",
		target.Name, target.Type, usage*100, usedGB, totalGB)
	fields := map[string]any{
		"storageTargetId":   target.ID,
		"storageTargetName": target.Name,
		"storageType":       target.Type,
		"usageRate":         usage,
		"usedBytes":         *info.Used,
		"totalBytes":        *info.Total,
	}
	_ = dispatcher.DispatchEvent(ctx, model.NotificationEventStorageCapacity, title, body, fields)
}

// runHealthCheckOnce 对所有启用目标执行一次连接测试并按需派发事件。
// "健康→故障"边沿触发告警；"故障→健康"边沿清除 notified 记忆，允许下次故障再次告警。
func (s *StorageTargetService) runHealthCheckOnce(ctx context.Context, dispatcher EventDispatcher, mu *sync.Mutex, notified map[uint]bool) {
	targets, err := s.targets.List(ctx)
	if err != nil {
		return
	}
	for i := range targets {
		target := targets[i]
		if !target.Enabled {
			continue
		}
		previousStatus := target.LastTestStatus
		configMap := map[string]any{}
		if err := s.cipher.DecryptJSON(target.ConfigCiphertext, &configMap); err != nil {
			continue
		}
		provider, err := s.registry.Create(ctx, storage.ParseProviderType(target.Type), configMap)
		now := time.Now().UTC()
		if err != nil {
			s.applyHealthResult(ctx, &target, now, false, err.Error())
			s.notifyUnhealthyTransition(ctx, dispatcher, mu, notified, &target, previousStatus, err.Error())
			continue
		}
		testErr := provider.TestConnection(ctx)
		if testErr != nil {
			s.applyHealthResult(ctx, &target, now, false, testErr.Error())
			s.notifyUnhealthyTransition(ctx, dispatcher, mu, notified, &target, previousStatus, testErr.Error())
			continue
		}
		s.applyHealthResult(ctx, &target, now, true, "连接成功")
		// 恢复健康：清除告警记忆
		mu.Lock()
		delete(notified, target.ID)
		mu.Unlock()
	}
}

func (s *StorageTargetService) applyHealthResult(ctx context.Context, target *model.StorageTarget, at time.Time, healthy bool, message string) {
	target.LastTestedAt = &at
	if healthy {
		target.LastTestStatus = "success"
	} else {
		target.LastTestStatus = "failed"
	}
	target.LastTestMessage = sanitizeMessage(message)
	_ = s.targets.Update(ctx, target)
}

func (s *StorageTargetService) notifyUnhealthyTransition(ctx context.Context, dispatcher EventDispatcher, mu *sync.Mutex, notified map[uint]bool, target *model.StorageTarget, previousStatus string, message string) {
	if dispatcher == nil {
		return
	}
	mu.Lock()
	already := notified[target.ID]
	if !already {
		notified[target.ID] = true
	}
	mu.Unlock()
	// 仅在上次状态是 success / unknown 且本次是 failed 时首次告警；
	// 已告警过的持续故障不重复发送（等 resetInterval 或恢复后重新触发）。
	if already {
		return
	}
	_ = previousStatus // 保留参数便于未来扩展：区分"从未测试"与"从 success 掉线"
	title := "BackupX 存储目标连接失败"
	body := fmt.Sprintf("存储目标：%s (类型: %s)\n错误：%s", target.Name, target.Type, message)
	fields := map[string]any{
		"storageTargetId":   target.ID,
		"storageTargetName": target.Name,
		"storageType":       target.Type,
		"error":             message,
	}
	_ = dispatcher.DispatchEvent(ctx, model.NotificationEventStorageUnhealthy, title, body, fields)
}

func (s *StorageTargetService) StartGoogleDriveOAuth(ctx context.Context, input GoogleDriveAuthStartInput, origin string) (*GoogleDriveAuthStartResult, error) {
	origin = normalizeOrigin(origin)
	if origin == "" {
		return nil, apperror.BadRequest("STORAGE_GOOGLE_OAUTH_ORIGIN_REQUIRED", "无法确定 Google Drive 回调地址", nil)
	}
	draft, err := s.buildGoogleDriveDraft(ctx, input, origin)
	if err != nil {
		return nil, err
	}
	payload, err := s.cipher.EncryptJSON(draft)
	if err != nil {
		return nil, apperror.Internal("STORAGE_GOOGLE_OAUTH_ENCRYPT_FAILED", "无法创建授权会话", err)
	}
	state, err := security.GenerateSecret(24)
	if err != nil {
		return nil, apperror.Internal("STORAGE_GOOGLE_OAUTH_STATE_FAILED", "无法生成授权状态", err)
	}
	expiresAt := time.Now().UTC().Add(10 * time.Minute)
	session := &model.OAuthSession{ProviderType: storage.TypeGoogleDrive, State: state, PayloadCiphertext: payload, TargetID: input.TargetID, ExpiresAt: expiresAt}
	if err := s.oauthSessions.Create(ctx, session); err != nil {
		return nil, apperror.Internal("STORAGE_GOOGLE_OAUTH_SESSION_FAILED", "无法创建授权会话", err)
	}
	oauthCfg := &oauth2.Config{ClientID: draft.ClientID, ClientSecret: draft.ClientSecret, RedirectURL: draft.RedirectURI, Endpoint: googleoauth.Endpoint, Scopes: []string{"https://www.googleapis.com/auth/drive"}}
	url := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	return &GoogleDriveAuthStartResult{AuthorizationURL: url, State: state, ExpiresAt: expiresAt}, nil
}

func (s *StorageTargetService) CompleteGoogleDriveOAuth(ctx context.Context, input GoogleDriveAuthCompleteInput) (*StorageTargetDetail, error) {
	session, err := s.oauthSessions.FindByState(ctx, strings.TrimSpace(input.State))
	if err != nil {
		return nil, apperror.Internal("STORAGE_GOOGLE_OAUTH_SESSION_FAILED", "无法读取授权会话", err)
	}
	if session == nil || session.UsedAt != nil || time.Now().UTC().After(session.ExpiresAt) {
		return nil, apperror.BadRequest("STORAGE_GOOGLE_OAUTH_STATE_INVALID", "Google Drive 授权状态无效或已过期", nil)
	}
	// Mark used immediately to prevent duplicate requests (e.g. React StrictMode double invocation)
	now := time.Now().UTC()
	session.UsedAt = &now
	_ = s.oauthSessions.Update(ctx, session)

	var draft googleDriveOAuthDraft
	if err := s.cipher.DecryptJSON(session.PayloadCiphertext, &draft); err != nil {
		return nil, apperror.Internal("STORAGE_GOOGLE_OAUTH_DECRYPT_FAILED", "无法读取授权会话内容", err)
	}
	oauthCfg := &oauth2.Config{ClientID: draft.ClientID, ClientSecret: draft.ClientSecret, RedirectURL: draft.RedirectURI, Endpoint: googleoauth.Endpoint, Scopes: []string{"https://www.googleapis.com/auth/drive"}}
	token, err := oauthCfg.Exchange(ctx, strings.TrimSpace(input.Code))
	if err != nil {
		return nil, apperror.BadRequest("STORAGE_GOOGLE_OAUTH_EXCHANGE_FAILED", "Google Drive 授权码换取失败", err)
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return nil, apperror.BadRequest("STORAGE_GOOGLE_OAUTH_REFRESH_TOKEN_MISSING", "未获取到 Google Drive refresh token，请重新授权", nil)
	}
	configMap := map[string]any{
		"clientId":     draft.ClientID,
		"clientSecret": draft.ClientSecret,
		"refreshToken": token.RefreshToken,
		"folderId":     draft.FolderID,
		"redirectUri":  draft.RedirectURI,
	}
	payload := StorageTargetUpsertInput{Name: draft.Name, Type: storage.TypeGoogleDrive, Description: draft.Description, Enabled: draft.Enabled, Config: configMap}
	var detail *StorageTargetDetail
	if session.TargetID != nil {
		detail, err = s.Update(ctx, *session.TargetID, payload)
	} else {
		detail, err = s.Create(ctx, payload)
	}
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *StorageTargetService) GoogleDriveProfile(ctx context.Context, id uint) (map[string]any, error) {
	detail, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if detail.Type != storage.TypeGoogleDrive {
		return nil, apperror.BadRequest("STORAGE_GOOGLE_DRIVE_TYPE_MISMATCH", "目标不是 Google Drive 存储类型", nil)
	}
	stored, err := s.targets.FindByID(ctx, id)
	if err != nil || stored == nil {
		return nil, apperror.New(http.StatusNotFound, "STORAGE_TARGET_NOT_FOUND", "存储目标不存在", err)
	}
	var cfg storage.GoogleDriveConfig
	if err := s.cipher.DecryptJSON(stored.ConfigCiphertext, &cfg); err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_DECRYPT_FAILED", "无法解密存储目标配置", err)
	}
	cfg = cfg.Normalize()
	oauthCfg := &oauth2.Config{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Endpoint: googleoauth.Endpoint, RedirectURL: cfg.RedirectURL, Scopes: []string{"https://www.googleapis.com/auth/drive"}}
	tokenSource := oauthCfg.TokenSource(ctx, &oauth2.Token{RefreshToken: cfg.RefreshToken, Expiry: time.Now().Add(-time.Hour)})
	client, err := goauth2api.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, apperror.BadRequest("STORAGE_GOOGLE_PROFILE_FAILED", "无法获取 Google Drive 用户信息", err)
	}
	userInfo, err := client.Userinfo.Get().Do()
	if err != nil {
		return nil, apperror.BadRequest("STORAGE_GOOGLE_PROFILE_FAILED", "无法获取 Google Drive 用户信息", err)
	}
	return map[string]any{"email": userInfo.Email, "name": userInfo.Name, "picture": userInfo.Picture}, nil
}

func (s *StorageTargetService) buildStorageTargetForTest(ctx context.Context, input StorageTargetTestInput) (*model.StorageTarget, error) {
	if input.TargetID == nil {
		return s.buildStorageTarget(ctx, nil, input.Payload)
	}
	existing, err := s.targets.FindByID(ctx, *input.TargetID)
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_GET_FAILED", "无法获取存储目标详情", err)
	}
	if existing == nil {
		return nil, apperror.New(http.StatusNotFound, "STORAGE_TARGET_NOT_FOUND", "存储目标不存在", fmt.Errorf("storage target %d not found", *input.TargetID))
	}
	if strings.TrimSpace(input.Payload.Type) == "" && strings.TrimSpace(input.Payload.Name) == "" && len(input.Payload.Config) == 0 {
		return existing, nil
	}
	item, err := s.buildStorageTarget(ctx, existing, input.Payload)
	if err != nil {
		return nil, err
	}
	item.ID = existing.ID
	item.LastTestedAt = existing.LastTestedAt
	item.LastTestStatus = existing.LastTestStatus
	item.LastTestMessage = existing.LastTestMessage
	return item, nil
}

func (s *StorageTargetService) buildStorageTarget(ctx context.Context, existing *model.StorageTarget, input StorageTargetUpsertInput) (*model.StorageTarget, error) {
	configMap, err := s.prepareConfig(ctx, existing, input)
	if err != nil {
		return nil, err
	}
	ciphertext, err := s.cipher.EncryptJSON(configMap)
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_ENCRYPT_FAILED", "无法保存存储目标配置", err)
	}
	quota := input.QuotaBytes
	if quota < 0 {
		quota = 0
	}
	item := &model.StorageTarget{
		Name:             strings.TrimSpace(input.Name),
		Type:             input.Type,
		Description:      strings.TrimSpace(input.Description),
		Enabled:          input.Enabled,
		ConfigCiphertext: ciphertext,
		ConfigVersion:    1,
		LastTestStatus:   "unknown",
		QuotaBytes:       quota,
	}
	if existing != nil {
		item.LastTestedAt = existing.LastTestedAt
		item.LastTestStatus = existing.LastTestStatus
		item.LastTestMessage = existing.LastTestMessage
		if existing.Type == input.Type {
			item.ConfigVersion = existing.ConfigVersion
		}
	}
	return item, nil
}

func (s *StorageTargetService) prepareConfig(ctx context.Context, existing *model.StorageTarget, input StorageTargetUpsertInput) (map[string]any, error) {
	if err := s.validateType(input.Type); err != nil {
		return nil, err
	}
	configMap := cloneMap(input.Config)
	if existing != nil {
		if existing.Type != input.Type {
			return nil, apperror.BadRequest("STORAGE_TARGET_TYPE_IMMUTABLE", "不支持直接修改存储目标类型", nil)
		}
		existingMap, err := s.decryptTargetConfig(existing)
		if err != nil {
			return nil, apperror.Internal("STORAGE_TARGET_DECRYPT_FAILED", "无法读取现有存储目标配置", err)
		}
		configMap = codec.MergeMaskedConfig(configMap, existingMap, s.registry.SensitiveFields(storage.ParseProviderType(input.Type)))
	}
	if _, err := s.registry.Create(ctx, storage.ParseProviderType(input.Type), configMap); err != nil {
		return nil, apperror.BadRequest("STORAGE_TARGET_INVALID_CONFIG", sanitizeMessage(err.Error()), err)
	}
	return configMap, nil
}

func (s *StorageTargetService) decryptTargetConfig(item *model.StorageTarget) (map[string]any, error) {
	var configMap map[string]any
	if err := s.cipher.DecryptJSON(item.ConfigCiphertext, &configMap); err != nil {
		return nil, err
	}
	return configMap, nil
}

func (s *StorageTargetService) buildGoogleDriveDraft(ctx context.Context, input GoogleDriveAuthStartInput, origin string) (*googleDriveOAuthDraft, error) {
	draft := &googleDriveOAuthDraft{
		TargetID:     input.TargetID,
		Name:         strings.TrimSpace(input.Name),
		Description:  strings.TrimSpace(input.Description),
		Enabled:      input.Enabled,
		ClientID:     strings.TrimSpace(input.ClientID),
		ClientSecret: strings.TrimSpace(input.ClientSecret),
		FolderID:     strings.TrimSpace(input.FolderID),
		RedirectURI:  strings.TrimRight(origin, "/") + "/storage-targets/google-drive/callback",
	}
	if input.TargetID == nil {
		if draft.Name == "" || draft.ClientID == "" || draft.ClientSecret == "" {
			return nil, apperror.BadRequest("STORAGE_GOOGLE_OAUTH_INVALID", "Google Drive 授权参数不完整", nil)
		}
		return draft, nil
	}
	existing, err := s.targets.FindByID(ctx, *input.TargetID)
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_GET_FAILED", "无法获取存储目标详情", err)
	}
	if existing == nil {
		return nil, apperror.New(http.StatusNotFound, "STORAGE_TARGET_NOT_FOUND", "存储目标不存在", fmt.Errorf("storage target %d not found", *input.TargetID))
	}
	if existing.Type != storage.TypeGoogleDrive {
		return nil, apperror.BadRequest("STORAGE_GOOGLE_DRIVE_TYPE_MISMATCH", "目标不是 Google Drive 存储类型", nil)
	}
	var cfg storage.GoogleDriveConfig
	if err := s.cipher.DecryptJSON(existing.ConfigCiphertext, &cfg); err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_DECRYPT_FAILED", "无法解密存储目标配置", err)
	}
	cfg = cfg.Normalize()
	if draft.Name == "" {
		draft.Name = existing.Name
	}
	if draft.Description == "" {
		draft.Description = existing.Description
	}
	if draft.ClientID == "" || codec.IsMaskedString(draft.ClientID) {
		draft.ClientID = cfg.ClientID
	}
	if draft.ClientSecret == "" || codec.IsMaskedString(draft.ClientSecret) {
		draft.ClientSecret = cfg.ClientSecret
	}
	if draft.FolderID == "" {
		draft.FolderID = cfg.FolderID
	}
	if draft.Name == "" || draft.ClientID == "" || draft.ClientSecret == "" {
		return nil, apperror.BadRequest("STORAGE_GOOGLE_OAUTH_INVALID", "Google Drive 授权参数不完整", nil)
	}
	return draft, nil
}

func (s *StorageTargetService) validateType(providerType string) error {
	if _, ok := s.registry.Factory(storage.ParseProviderType(providerType)); !ok {
		return apperror.BadRequest("STORAGE_PROVIDER_UNSUPPORTED", "不支持的存储类型", fmt.Errorf("provider %s not found", providerType))
	}
	return nil
}

func toStorageTargetSummary(item *model.StorageTarget) StorageTargetSummary {
	return StorageTargetSummary{
		ID:              item.ID,
		Name:            item.Name,
		Type:            item.Type,
		Description:     item.Description,
		Enabled:         item.Enabled,
		Starred:         item.Starred,
		ConfigVersion:   item.ConfigVersion,
		LastTestedAt:    item.LastTestedAt,
		LastTestStatus:  item.LastTestStatus,
		LastTestMessage: item.LastTestMessage,
		QuotaBytes:      item.QuotaBytes,
		UpdatedAt:       item.UpdatedAt,
	}
}

func sanitizeMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "操作失败"
	}
	if len(message) > 255 {
		return message[:255]
	}
	return message
}

func normalizeOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	return strings.TrimRight(origin, "/")
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

type StorageTargetUsage struct {
	TargetID    uint                      `json:"targetId"`
	TargetName  string                    `json:"targetName"`
	RecordCount int64                     `json:"recordCount"`
	TotalSize   int64                     `json:"totalSize"`
	DiskUsage   *storage.StorageUsageInfo `json:"diskUsage,omitempty"`
}

func (s *StorageTargetService) GetUsage(ctx context.Context, id uint) (*StorageTargetUsage, error) {
	target, err := s.targets.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("STORAGE_TARGET_GET_FAILED", "无法获取存储目标详情", err)
	}
	if target == nil {
		return nil, apperror.New(http.StatusNotFound, "STORAGE_TARGET_NOT_FOUND", "存储目标不存在", fmt.Errorf("storage target %d not found", id))
	}
	result := &StorageTargetUsage{TargetID: id, TargetName: target.Name}
	if s.records != nil {
		usageItems, usageErr := s.records.StorageUsage(ctx)
		if usageErr == nil {
			for _, item := range usageItems {
				if item.StorageTargetID == id {
					result.TotalSize = item.TotalSize
					break
				}
			}
		}
	}
	// 尝试查询远端真实存储空间（部分后端如 local/Google Drive/WebDAV 支持）
	configMap := map[string]any{}
	if decryptErr := s.cipher.DecryptJSON(target.ConfigCiphertext, &configMap); decryptErr == nil {
		if provider, createErr := s.registry.Create(ctx, target.Type, configMap); createErr == nil {
			if abouter, ok := provider.(storage.StorageAbout); ok {
				if diskUsage, aboutErr := abouter.About(ctx); aboutErr == nil {
					result.DiskUsage = diskUsage
				}
			}
		}
	}
	return result, nil
}
