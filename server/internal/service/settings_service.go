package service

import (
	"context"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

type SettingsService struct {
	configs repository.SystemConfigRepository
}

func NewSettingsService(configs repository.SystemConfigRepository) *SettingsService {
	return &SettingsService{configs: configs}
}

// settingsKeys lists all user-editable setting keys.
var settingsKeys = []string{
	"site_name",
	"language",
	"timezone",
	"backup_notification_enabled",
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
	for key, value := range settings {
		if !allowed[key] {
			continue
		}
		item := &model.SystemConfig{Key: key, Value: value}
		if err := s.configs.Upsert(ctx, item); err != nil {
			return nil, apperror.Internal("SETTINGS_UPDATE_FAILED", "无法更新系统设置", err)
		}
	}
	return s.GetAll(ctx)
}
