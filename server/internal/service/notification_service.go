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
	Name      string `json:"name" binding:"required,min=1,max=100"`
	Type      string `json:"type" binding:"required,oneof=email webhook telegram"`
	Enabled   bool   `json:"enabled"`
	OnSuccess bool   `json:"onSuccess"`
	OnFailure bool   `json:"onFailure"`
	// EventTypes 订阅的扩展事件列表。与 OnSuccess/OnFailure 并存：
	//   - 两者均空时，订阅"备份成功/失败"对应原有语义（兼容）。
	//   - EventTypes 显式指定时优先按清单匹配。
	EventTypes []string       `json:"eventTypes"`
	Config     map[string]any `json:"config" binding:"required"`
}

type NotificationSummary struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Enabled    bool      `json:"enabled"`
	OnSuccess  bool      `json:"onSuccess"`
	OnFailure  bool      `json:"onFailure"`
	EventTypes []string  `json:"eventTypes"`
	UpdatedAt  time.Time `json:"updatedAt"`
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
	// broadcaster 可选：用于同步把事件推送给 SSE 订阅者（Dashboard 实时刷新）
	broadcaster *EventBroadcaster
}

// SetBroadcaster 注入事件广播器，每次 DispatchEvent 同时走 SSE 实时通道。
func (s *NotificationService) SetBroadcaster(b *EventBroadcaster) {
	s.broadcaster = b
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
	eventType := model.NotificationEventBackupFailed
	if success {
		eventType = model.NotificationEventBackupSuccess
	}
	items, err := s.collectSubscribers(ctx, eventType, success)
	if err != nil {
		return err
	}
	message := buildNotificationMessage(event)
	message.Fields["eventType"] = eventType
	return s.deliver(ctx, items, message)
}

// DispatchEvent 面向任意企业级事件的通用分发入口。
//   - title / body / fields 构造通知内容
//   - eventType 对应 model.NotificationEvent* 常量，用于订阅匹配
//
// 订阅匹配规则：
//  1. notification.EventTypes 非空：必须包含 eventType
//  2. notification.EventTypes 为空：沿用 OnSuccess/OnFailure 开关（仅 backup_* 事件）
func (s *NotificationService) DispatchEvent(ctx context.Context, eventType string, title string, body string, fields map[string]any) error {
	// 同步广播到 SSE 订阅者（前端 Dashboard 实时推送）。
	// 非阻塞：即便广播器未注入或订阅者已满也不影响 Notification 持久渠道。
	if s.broadcaster != nil {
		_ = s.broadcaster.Publish(ctx, eventType, title, body, fields)
	}
	// 将 fallback 布尔用于旧语义场景（backup_success / backup_failed）。
	fallbackSuccess := eventType == model.NotificationEventBackupSuccess
	items, err := s.collectSubscribers(ctx, eventType, fallbackSuccess)
	if err != nil {
		return err
	}
	if fields == nil {
		fields = map[string]any{}
	}
	fields["eventType"] = eventType
	fields["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	message := notify.Message{Title: title, Body: body, Fields: fields}
	return s.deliver(ctx, items, message)
}

func (s *NotificationService) SendAuthEmailOTP(ctx context.Context, to string, code string) error {
	return s.sendFirstByType(ctx, "email", map[string]any{"to": strings.TrimSpace(to)}, notify.Message{
		Title: "BackupX 登录验证码",
		Body:  fmt.Sprintf("您的 BackupX 登录验证码为：%s\n验证码 5 分钟内有效。若非本人操作，请立即检查账号安全。", code),
		Fields: map[string]any{
			"purpose": "login_otp",
		},
	})
}

func (s *NotificationService) SendAuthSMSOTP(ctx context.Context, phone string, code string) error {
	return s.sendFirstByType(ctx, "webhook", nil, notify.Message{
		Title: "BackupX 登录验证码",
		Body:  fmt.Sprintf("BackupX 登录验证码：%s，5 分钟内有效。", code),
		Fields: map[string]any{
			"phone":   strings.TrimSpace(phone),
			"code":    code,
			"purpose": "login_otp",
		},
	})
}

func (s *NotificationService) sendFirstByType(ctx context.Context, notificationType string, override map[string]any, message notify.Message) error {
	items, err := s.notifications.List(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if !item.Enabled || item.Type != notificationType {
			continue
		}
		configMap := map[string]any{}
		if err := s.cipher.DecryptJSON(item.ConfigCiphertext, &configMap); err != nil {
			return fmt.Errorf("decrypt notification %d config: %w", item.ID, err)
		}
		for key, value := range override {
			configMap[key] = value
		}
		return s.registry.Send(ctx, item.Type, configMap, message)
	}
	return fmt.Errorf("no enabled %s notification configured", notificationType)
}

// collectSubscribers 按事件类型收集启用的订阅者。
// 列出启用通知后按事件类型再过滤（避免引入新 repository 方法）。
func (s *NotificationService) collectSubscribers(ctx context.Context, eventType string, fallbackSuccess bool) ([]model.Notification, error) {
	all, err := s.notifications.List(ctx)
	if err != nil {
		return nil, err
	}
	matched := make([]model.Notification, 0, len(all))
	for _, item := range all {
		if !item.Enabled {
			continue
		}
		events := decodeEventTypes(item.EventTypes)
		if len(events) > 0 {
			if !containsString(events, eventType) {
				continue
			}
		} else {
			// 旧语义兼容：仅对 backup_success / backup_failed 走 OnSuccess/OnFailure
			switch eventType {
			case model.NotificationEventBackupSuccess:
				if !item.OnSuccess {
					continue
				}
			case model.NotificationEventBackupFailed:
				if !item.OnFailure {
					continue
				}
			default:
				// 其他事件类型必须显式订阅才推送
				continue
			}
			// 额外校验 fallbackSuccess 参数，保持历史行为一致
			_ = fallbackSuccess
		}
		matched = append(matched, item)
	}
	return matched, nil
}

func (s *NotificationService) deliver(ctx context.Context, items []model.Notification, message notify.Message) error {
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

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
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
	item := &model.Notification{
		Name:             strings.TrimSpace(input.Name),
		Type:             strings.TrimSpace(input.Type),
		ConfigCiphertext: ciphertext,
		Enabled:          input.Enabled,
		OnSuccess:        input.OnSuccess,
		OnFailure:        input.OnFailure,
		EventTypes:       encodeEventTypes(input.EventTypes),
	}
	return item, nil
}

// encodeEventTypes 把事件切片规范化为逗号分隔字符串（去重+trim）。
func encodeEventTypes(events []string) string {
	seen := map[string]bool{}
	out := make([]string, 0, len(events))
	for _, e := range events {
		trimmed := strings.TrimSpace(e)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return strings.Join(out, ",")
}

// decodeEventTypes 解析存储字符串为切片。
func decodeEventTypes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
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
	return NotificationSummary{
		ID:         item.ID,
		Name:       item.Name,
		Type:       item.Type,
		Enabled:    item.Enabled,
		OnSuccess:  item.OnSuccess,
		OnFailure:  item.OnFailure,
		EventTypes: decodeEventTypes(item.EventTypes),
		UpdatedAt:  item.UpdatedAt,
	}
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
