package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/notify"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage/codec"
)

type NotificationUpsertInput struct {
	Name      string         `json:"name" binding:"required,min=1,max=100"`
	Type      string         `json:"type" binding:"required,oneof=email webhook telegram"`
	Enabled   bool           `json:"enabled"`
	OnSuccess bool           `json:"onSuccess"`
	OnFailure bool           `json:"onFailure"`
	Config    map[string]any `json:"config" binding:"required"`
}

type NotificationSummary struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Enabled   bool      `json:"enabled"`
	OnSuccess bool      `json:"onSuccess"`
	OnFailure bool      `json:"onFailure"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type NotificationDetail struct {
	NotificationSummary
	Config       map[string]any `json:"config"`
	MaskedFields []string       `json:"maskedFields,omitempty"`
}

type NotificationService struct {
	notifications repository.NotificationRepository
	registry      *notify.Registry
	cipher        *codec.ConfigCipher
}

func NewNotificationService(notifications repository.NotificationRepository, registry *notify.Registry, cipher *codec.ConfigCipher) *NotificationService {
	return &NotificationService{notifications: notifications, registry: registry, cipher: cipher}
}

func (s *NotificationService) List(ctx context.Context) ([]NotificationSummary, error) {
	items, err := s.notifications.List(ctx)
	if err != nil {
		return nil, apperror.Internal("NOTIFICATION_LIST_FAILED", "无法获取通知配置列表", err)
	}
	result := make([]NotificationSummary, 0, len(items))
	for _, item := range items {
		result = append(result, toNotificationSummary(&item))
	}
	return result, nil
}

func (s *NotificationService) Get(ctx context.Context, id uint) (*NotificationDetail, error) {
	item, err := s.notifications.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("NOTIFICATION_GET_FAILED", "无法获取通知配置详情", err)
	}
	if item == nil {
		return nil, apperror.New(http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "通知配置不存在", fmt.Errorf("notification %d not found", id))
	}
	return s.toDetail(item)
}

func (s *NotificationService) Create(ctx context.Context, input NotificationUpsertInput) (*NotificationDetail, error) {
	if err := s.validateInput(ctx, 0, input); err != nil {
		return nil, err
	}
	item, err := s.buildNotification(nil, input)
	if err != nil {
		return nil, err
	}
	if err := s.notifications.Create(ctx, item); err != nil {
		return nil, apperror.Internal("NOTIFICATION_CREATE_FAILED", "无法创建通知配置", err)
	}
	return s.Get(ctx, item.ID)
}

func (s *NotificationService) Update(ctx context.Context, id uint, input NotificationUpsertInput) (*NotificationDetail, error) {
	existing, err := s.notifications.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("NOTIFICATION_GET_FAILED", "无法获取通知配置详情", err)
	}
	if existing == nil {
		return nil, apperror.New(http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "通知配置不存在", fmt.Errorf("notification %d not found", id))
	}
	if err := s.validateInput(ctx, existing.ID, input); err != nil {
		return nil, err
	}
	item, err := s.buildNotification(existing, input)
	if err != nil {
		return nil, err
	}
	item.ID = existing.ID
	item.CreatedAt = existing.CreatedAt
	if err := s.notifications.Update(ctx, item); err != nil {
		return nil, apperror.Internal("NOTIFICATION_UPDATE_FAILED", "无法更新通知配置", err)
	}
	return s.Get(ctx, id)
}

func (s *NotificationService) Delete(ctx context.Context, id uint) error {
	item, err := s.notifications.FindByID(ctx, id)
	if err != nil {
		return apperror.Internal("NOTIFICATION_GET_FAILED", "无法获取通知配置详情", err)
	}
	if item == nil {
		return apperror.New(http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "通知配置不存在", fmt.Errorf("notification %d not found", id))
	}
	if err := s.notifications.Delete(ctx, id); err != nil {
		return apperror.Internal("NOTIFICATION_DELETE_FAILED", "无法删除通知配置", err)
	}
	return nil
}

func (s *NotificationService) Test(ctx context.Context, input NotificationUpsertInput) error {
	if err := s.registry.Validate(strings.TrimSpace(input.Type), input.Config); err != nil {
		return apperror.BadRequest("NOTIFICATION_INVALID", "通知配置不合法", err)
	}
	message := notify.Message{Title: "BackupX 通知测试", Body: "这是一条来自 BackupX 的测试通知。", Fields: map[string]any{"type": input.Type, "timestamp": time.Now().UTC().Format(time.RFC3339)}}
	if err := s.registry.Send(ctx, input.Type, input.Config, message); err != nil {
		return apperror.BadRequest("NOTIFICATION_TEST_FAILED", "发送测试通知失败", err)
	}
	return nil
}

func (s *NotificationService) TestSaved(ctx context.Context, id uint) error {
	item, err := s.notifications.FindByID(ctx, id)
	if err != nil {
		return apperror.Internal("NOTIFICATION_GET_FAILED", "无法获取通知配置", err)
	}
	if item == nil {
		return apperror.New(http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "通知配置不存在", fmt.Errorf("notification %d not found", id))
	}
	configMap := map[string]any{}
	if err := s.cipher.DecryptJSON(item.ConfigCiphertext, &configMap); err != nil {
		return apperror.Internal("NOTIFICATION_DECRYPT_FAILED", "无法读取通知配置", err)
	}
	message := notify.Message{Title: "BackupX 通知测试", Body: "这是一条来自 BackupX 的测试通知。", Fields: map[string]any{"type": item.Type, "timestamp": time.Now().UTC().Format(time.RFC3339)}}
	if err := s.registry.Send(ctx, item.Type, configMap, message); err != nil {
		return apperror.BadRequest("NOTIFICATION_TEST_FAILED", "发送测试通知失败", err)
	}
	return nil
}

func (s *NotificationService) NotifyBackupResult(ctx context.Context, event BackupExecutionNotification) error {
	success := event.Error == nil && event.Record != nil && event.Record.Status == "success"
	items, err := s.notifications.ListEnabledForEvent(ctx, success)
	if err != nil {
		return err
	}
	message := buildNotificationMessage(event)
	var joined error
	for _, item := range items {
		configMap := map[string]any{}
		if err := s.cipher.DecryptJSON(item.ConfigCiphertext, &configMap); err != nil {
			joined = errors.Join(joined, fmt.Errorf("decrypt notification %d config: %w", item.ID, err))
			continue
		}
		if err := s.registry.Send(ctx, item.Type, configMap, message); err != nil {
			joined = errors.Join(joined, fmt.Errorf("send notification %s failed: %w", item.Name, err))
		}
	}
	return joined
}

func (s *NotificationService) validateInput(ctx context.Context, currentID uint, input NotificationUpsertInput) error {
	existing, err := s.notifications.FindByName(ctx, strings.TrimSpace(input.Name))
	if err != nil {
		return apperror.Internal("NOTIFICATION_LOOKUP_FAILED", "无法检查通知配置名称", err)
	}
	if existing != nil && existing.ID != currentID {
		return apperror.Conflict("NOTIFICATION_NAME_EXISTS", "通知配置名称已存在", nil)
	}
	if err := s.registry.Validate(strings.TrimSpace(input.Type), input.Config); err != nil {
		return apperror.BadRequest("NOTIFICATION_INVALID", "通知配置不合法", err)
	}
	return nil
}

func (s *NotificationService) buildNotification(existing *model.Notification, input NotificationUpsertInput) (*model.Notification, error) {
	configMap := input.Config
	if existing != nil {
		currentConfig := map[string]any{}
		if err := s.cipher.DecryptJSON(existing.ConfigCiphertext, &currentConfig); err != nil {
			return nil, apperror.Internal("NOTIFICATION_DECRYPT_FAILED", "无法读取现有通知配置", err)
		}
		configMap = codec.MergeMaskedConfig(input.Config, currentConfig, s.registry.SensitiveFields(input.Type))
	}
	ciphertext, err := s.cipher.EncryptJSON(configMap)
	if err != nil {
		return nil, apperror.Internal("NOTIFICATION_ENCRYPT_FAILED", "无法保存通知配置", err)
	}
	item := &model.Notification{Name: strings.TrimSpace(input.Name), Type: strings.TrimSpace(input.Type), ConfigCiphertext: ciphertext, Enabled: input.Enabled, OnSuccess: input.OnSuccess, OnFailure: input.OnFailure}
	return item, nil
}

func (s *NotificationService) toDetail(item *model.Notification) (*NotificationDetail, error) {
	configMap := map[string]any{}
	if err := s.cipher.DecryptJSON(item.ConfigCiphertext, &configMap); err != nil {
		return nil, apperror.Internal("NOTIFICATION_DECRYPT_FAILED", "无法读取通知配置", err)
	}
	sensitiveFields := s.registry.SensitiveFields(item.Type)
	return &NotificationDetail{NotificationSummary: toNotificationSummary(item), Config: codec.MaskConfig(configMap, sensitiveFields), MaskedFields: sensitiveFields}, nil
}

func toNotificationSummary(item *model.Notification) NotificationSummary {
	return NotificationSummary{ID: item.ID, Name: item.Name, Type: item.Type, Enabled: item.Enabled, OnSuccess: item.OnSuccess, OnFailure: item.OnFailure, UpdatedAt: item.UpdatedAt}
}

func buildNotificationMessage(event BackupExecutionNotification) notify.Message {
	statusText := "失败"
	if event.Error == nil && event.Record != nil && event.Record.Status == "success" {
		statusText = "成功"
	}
	taskName := "未知任务"
	if event.Task != nil {
		taskName = event.Task.Name
	}
	body := fmt.Sprintf("任务：%s\n状态：%s", taskName, statusText)
	fields := map[string]any{"taskName": taskName, "status": statusText}
	if event.Record != nil {
		body += fmt.Sprintf("\n开始时间：%s\n耗时：%d 秒", event.Record.StartedAt.Format(time.RFC3339), event.Record.DurationSeconds)
		fields["recordId"] = event.Record.ID
		fields["durationSeconds"] = event.Record.DurationSeconds
		if event.Record.FileName != "" {
			body += fmt.Sprintf("\n文件：%s", event.Record.FileName)
			fields["fileName"] = event.Record.FileName
		}
		if event.Record.FileSize > 0 {
			body += fmt.Sprintf("\n大小：%d", event.Record.FileSize)
			fields["fileSize"] = event.Record.FileSize
		}
		if event.Record.ErrorMessage != "" {
			body += fmt.Sprintf("\n错误：%s", event.Record.ErrorMessage)
			fields["error"] = event.Record.ErrorMessage
		}
	}
	return notify.Message{Title: "BackupX 备份" + statusText + "通知", Body: body, Fields: fields}
}
