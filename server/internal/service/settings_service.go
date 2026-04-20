package service

import (
	"context"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// AuditWebhookConfigurer 抽象审计 webhook 配置接口，由 AuditService 实现。
// 用接口解耦避免 settings_service 直接依赖 AuditService 具体类型。
type AuditWebhookConfigurer interface {
	SetWebhook(url, secret string)
}

type SettingsService struct {
	configs      repository.SystemConfigRepository
	auditWebhook AuditWebhookConfigurer
}

func NewSettingsService(configs repository.SystemConfigRepository) *SettingsService {
	return &SettingsService{configs: configs}
}

// SetAuditWebhookConfigurer 注入 audit webhook 配置接收方。
// 启动时立即用当前 DB 中的设置调用一次，后续每次 Update 变更 webhook key 时同步推送。
func (s *SettingsService) SetAuditWebhookConfigurer(ctx context.Context, configurer AuditWebhookConfigurer) {
	if s == nil || configurer == nil {
		return
	}
	s.auditWebhook = configurer
	// 启动时同步一次，保证重启后配置不丢失
	all, err := s.GetAll(ctx)
	if err == nil {
		configurer.SetWebhook(all[SettingKeyAuditWebhookURL], all[SettingKeyAuditWebhookSecret])
	}
}

// 可被前端写入的系统设置键。新增键必须同步加入此清单，
// 否则 Update 会忽略（安全原则：显式 allow-list）。
const (
	SettingKeySiteName                  = "site_name"
	SettingKeyLanguage                  = "language"
	SettingKeyTimezone                  = "timezone"
	SettingKeyBackupNotificationEnabled = "backup_notification_enabled"
	SettingKeyBandwidthLimit            = "bandwidth_limit"
	SettingKeyAuditWebhookURL           = "audit_webhook_url"
	SettingKeyAuditWebhookSecret        = "audit_webhook_secret"
)

var settingsKeys = []string{
	SettingKeySiteName,
	SettingKeyLanguage,
	SettingKeyTimezone,
	SettingKeyBackupNotificationEnabled,
	SettingKeyBandwidthLimit,
	SettingKeyAuditWebhookURL,
	SettingKeyAuditWebhookSecret,
}

func (s *SettingsService) GetAll(ctx context.Context) (map[string]string, error) {
	items, err := s.configs.List(ctx)
	if err != nil {
		return nil, apperror.Internal("SETTINGS_LIST_FAILED", "无法获取系统设置", err)
	}
	result := make(map[string]string, len(items))
	for _, item := range items {
		result[item.Key] = item.Value
	}
	return result, nil
}

func (s *SettingsService) Update(ctx context.Context, settings map[string]string) (map[string]string, error) {
	allowed := make(map[string]bool, len(settingsKeys))
	for _, key := range settingsKeys {
		allowed[key] = true
	}
	auditWebhookTouched := false
	for key, value := range settings {
		if !allowed[key] {
			continue
		}
		item := &model.SystemConfig{Key: key, Value: value}
		if err := s.configs.Upsert(ctx, item); err != nil {
			return nil, apperror.Internal("SETTINGS_UPDATE_FAILED", "无法更新系统设置", err)
		}
		if key == SettingKeyAuditWebhookURL || key == SettingKeyAuditWebhookSecret {
			auditWebhookTouched = true
		}
	}
	// audit webhook 配置变化：立即同步到 AuditService，避免重启才生效
	if auditWebhookTouched && s.auditWebhook != nil {
		all, _ := s.GetAll(ctx)
		s.auditWebhook.SetWebhook(all[SettingKeyAuditWebhookURL], all[SettingKeyAuditWebhookSecret])
	}
	return s.GetAll(ctx)
}
