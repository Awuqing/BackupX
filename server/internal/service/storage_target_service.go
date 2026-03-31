package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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
	Type        string         `json:"type" binding:"required,oneof=local_disk google_drive s3 webdav"`
	Description string         `json:"description" binding:"max=255"`
	Enabled     bool           `json:"enabled"`
	Config      map[string]any `json:"config" binding:"required"`
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
	item := &model.StorageTarget{
		Name:             strings.TrimSpace(input.Name),
		Type:             input.Type,
		Description:      strings.TrimSpace(input.Description),
		Enabled:          input.Enabled,
		ConfigCiphertext: ciphertext,
		ConfigVersion:    1,
		LastTestStatus:   "unknown",
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
