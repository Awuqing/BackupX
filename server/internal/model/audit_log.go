package model

import "time"

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"column:user_id;index" json:"userId"`
	Username   string    `gorm:"column:username;size:64;not null" json:"username"`
	Category   string    `gorm:"column:category;size:32;index;not null" json:"category"`
	Action     string    `gorm:"column:action;size:64;not null" json:"action"`
	TargetType string    `gorm:"column:target_type;size:32" json:"targetType"`
	TargetID   string    `gorm:"column:target_id;size:64" json:"targetId"`
	TargetName string    `gorm:"column:target_name;size:128" json:"targetName"`
	Detail     string    `gorm:"column:detail;type:text" json:"detail"`
	ClientIP   string    `gorm:"column:client_ip;size:45" json:"clientIp"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
