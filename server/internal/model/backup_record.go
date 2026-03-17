package model

import "time"

const (
	BackupRecordStatusRunning = "running"
	BackupRecordStatusSuccess = "success"
	BackupRecordStatusFailed  = "failed"
)

type BackupRecord struct {
	ID              uint          `gorm:"primaryKey" json:"id"`
	TaskID          uint          `gorm:"column:task_id;index;not null" json:"taskId"`
	Task            BackupTask    `json:"task,omitempty"`
	StorageTargetID uint          `gorm:"column:storage_target_id;index;not null" json:"storageTargetId"`
	StorageTarget   StorageTarget `json:"storageTarget,omitempty"`
	Status          string        `gorm:"size:20;index;not null" json:"status"`
	FileName        string        `gorm:"column:file_name;size:255" json:"fileName"`
	FileSize        int64         `gorm:"column:file_size;not null;default:0" json:"fileSize"`
	StoragePath     string        `gorm:"column:storage_path;size:500" json:"storagePath"`
	DurationSeconds int           `gorm:"column:duration_seconds;not null;default:0" json:"durationSeconds"`
	ErrorMessage    string        `gorm:"column:error_message;size:2000" json:"errorMessage"`
	LogContent      string        `gorm:"column:log_content;type:text" json:"logContent"`
	StartedAt       time.Time     `gorm:"column:started_at;index;not null" json:"startedAt"`
	CompletedAt     *time.Time    `gorm:"column:completed_at;index" json:"completedAt,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

func (BackupRecord) TableName() string {
	return "backup_records"
}
