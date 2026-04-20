package model

import "time"

// 通知事件类型（企业级事件总线）。
// 任一 Notification 可订阅多个事件，EventTypes 字段存 CSV。
// 空 EventTypes + OnSuccess/OnFailure=true 时沿用旧语义（仅备份成功/失败）。
const (
	NotificationEventBackupSuccess = "backup_success"
	NotificationEventBackupFailed  = "backup_failed"
	NotificationEventRestoreSuccess = "restore_success"
	NotificationEventRestoreFailed  = "restore_failed"
	NotificationEventVerifyFailed  = "verify_failed"
	NotificationEventSLAViolation  = "sla_violation"
	// NotificationEventStorageUnhealthy 存储目标连接失败（后台健康扫描触发）。
	NotificationEventStorageUnhealthy = "storage_unhealthy"
	// NotificationEventReplicationFailed 备份复制失败。
	NotificationEventReplicationFailed = "replication_failed"
	// NotificationEventAgentOutdated Agent 版本落后 Master，建议升级。
	NotificationEventAgentOutdated = "agent_outdated"
	// NotificationEventStorageCapacity 存储目标使用率超过预警阈值（85%）。
	NotificationEventStorageCapacity = "storage_capacity_warning"
)

type Notification struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Type             string    `gorm:"size:20;index;not null" json:"type"`
	Name             string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	ConfigCiphertext string    `gorm:"column:config_ciphertext;type:text;not null" json:"-"`
	Enabled          bool      `gorm:"not null;default:true" json:"enabled"`
	OnSuccess        bool      `gorm:"column:on_success;not null;default:false" json:"onSuccess"`
	OnFailure        bool      `gorm:"column:on_failure;not null;default:true" json:"onFailure"`
	// EventTypes 逗号分隔，订阅的事件类型。
	// 空 = 仅监听备份成功/失败（兼容旧配置）；非空则严格按订阅触发。
	EventTypes string    `gorm:"column:event_types;size:500" json:"eventTypes"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (Notification) TableName() string {
	return "notifications"
}
