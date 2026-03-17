package model

import "time"

type StorageTarget struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	Name             string     `gorm:"size:128;uniqueIndex;not null" json:"name"`
	Type             string     `gorm:"size:32;index;not null" json:"type"`
	Description      string     `gorm:"size:255" json:"description"`
	Enabled          bool       `gorm:"not null;default:true" json:"enabled"`
	ConfigCiphertext string     `gorm:"column:config_ciphertext;type:text;not null" json:"-"`
	ConfigVersion    int        `gorm:"not null;default:1" json:"configVersion"`
	LastTestedAt     *time.Time `gorm:"column:last_tested_at" json:"lastTestedAt,omitempty"`
	LastTestStatus   string     `gorm:"column:last_test_status;size:32;not null;default:'unknown'" json:"lastTestStatus"`
	LastTestMessage  string     `gorm:"column:last_test_message;size:512" json:"lastTestMessage"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

func (StorageTarget) TableName() string {
	return "storage_targets"
}
