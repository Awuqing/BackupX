package model

import "time"

type OAuthSession struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	ProviderType      string     `gorm:"column:provider_type;size:32;index;not null" json:"providerType"`
	State             string     `gorm:"size:255;uniqueIndex;not null" json:"state"`
	PayloadCiphertext string     `gorm:"column:payload_ciphertext;type:text;not null" json:"-"`
	TargetID          *uint      `gorm:"column:target_id" json:"targetId,omitempty"`
	ExpiresAt         time.Time  `gorm:"column:expires_at;index;not null" json:"expiresAt"`
	UsedAt            *time.Time `gorm:"column:used_at" json:"usedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

func (OAuthSession) TableName() string {
	return "oauth_sessions"
}
