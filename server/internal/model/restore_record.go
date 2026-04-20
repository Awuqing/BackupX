package model

import "time"

// RestoreRecord 代表一次恢复执行，用于审计、实时日志与列表页。
// 每次从 BackupRecord 触发恢复都会产生独立 RestoreRecord，与 BackupRecord 一对多。
const (
	RestoreRecordStatusRunning = "running"
	RestoreRecordStatusSuccess = "success"
	RestoreRecordStatusFailed  = "failed"
)

type RestoreRecord struct {
	ID              uint         `gorm:"primaryKey" json:"id"`
	BackupRecordID  uint         `gorm:"column:backup_record_id;index;not null" json:"backupRecordId"`
	BackupRecord    BackupRecord `json:"backupRecord,omitempty"`
	TaskID          uint         `gorm:"column:task_id;index;not null" json:"taskId"`
	Task            BackupTask   `json:"task,omitempty"`
	NodeID          uint         `gorm:"column:node_id;index;default:0" json:"nodeId"`
	Status          string       `gorm:"size:20;index;not null" json:"status"`
	ErrorMessage    string       `gorm:"column:error_message;size:2000" json:"errorMessage"`
	LogContent      string       `gorm:"column:log_content;type:text" json:"logContent"`
	DurationSeconds int          `gorm:"column:duration_seconds;not null;default:0" json:"durationSeconds"`
	StartedAt       time.Time    `gorm:"column:started_at;index;not null" json:"startedAt"`
	CompletedAt     *time.Time   `gorm:"column:completed_at;index" json:"completedAt,omitempty"`
	TriggeredBy     string       `gorm:"column:triggered_by;size:100" json:"triggeredBy"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

func (RestoreRecord) TableName() string {
	return "restore_records"
}
