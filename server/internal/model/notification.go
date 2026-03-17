package model

import "time"

type Notification struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Type             string    `gorm:"size:20;index;not null" json:"type"`
	Name             string    `gorm:"size:100;uniqueIndex;not null" json:"name"`
	ConfigCiphertext string    `gorm:"column:config_ciphertext;type:text;not null" json:"-"`
	Enabled          bool      `gorm:"not null;default:true" json:"enabled"`
	OnSuccess        bool      `gorm:"column:on_success;not null;default:false" json:"onSuccess"`
	OnFailure        bool      `gorm:"column:on_failure;not null;default:true" json:"onFailure"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (Notification) TableName() string {
	return "notifications"
}
