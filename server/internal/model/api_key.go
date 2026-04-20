package model

import "time"

// ApiKey 用于 CI/CD、监控脚本等非交互式场景通过 HTTP API 访问 BackupX。
// 明文 Key 仅在创建时返回一次，数据库存储 SHA-256 哈希。
// 认证中间件：当 Authorization: Bearer 值以 "bax_" 前缀开头时走 API Key 验证。
type ApiKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Name       string     `gorm:"size:128;not null" json:"name"`
	Role       string     `gorm:"size:32;not null;default:viewer" json:"role"`
	KeyHash    string     `gorm:"column:key_hash;size:128;uniqueIndex;not null" json:"-"`
	Prefix     string     `gorm:"size:32;not null" json:"prefix"`
	CreatedBy  string     `gorm:"column:created_by;size:128" json:"createdBy"`
	LastUsedAt *time.Time `gorm:"column:last_used_at" json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `gorm:"column:expires_at" json:"expiresAt,omitempty"`
	Disabled   bool       `gorm:"not null;default:false" json:"disabled"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

func (ApiKey) TableName() string {
	return "api_keys"
}
