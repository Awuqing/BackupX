package model

import "time"

// VerificationRecord 记录一次备份验证（或演练）的执行。
// 验证目标：从指定 BackupRecord 读取归档 → 在沙箱内执行只读校验
// （解压/格式检查/完整性校验），不改动源数据。
const (
	VerificationRecordStatusRunning = "running"
	VerificationRecordStatusSuccess = "success"
	VerificationRecordStatusFailed  = "failed"

	// VerificationModeQuick 仅做格式与完整性校验（tar header、SHA-256、DB dump 头）。
	// 耗时短，不占用目标系统资源，适合每日调度。
	VerificationModeQuick = "quick"
	// VerificationModeDeep 真正恢复到隔离沙箱（临时库或解压目录），验证可读。
	// 耗时较长，适合每周/每月。当前版本保留接口不实现。
	VerificationModeDeep = "deep"
)

type VerificationRecord struct {
	ID              uint         `gorm:"primaryKey" json:"id"`
	BackupRecordID  uint         `gorm:"column:backup_record_id;index;not null" json:"backupRecordId"`
	BackupRecord    BackupRecord `json:"backupRecord,omitempty"`
	TaskID          uint         `gorm:"column:task_id;index;not null" json:"taskId"`
	Task            BackupTask   `json:"task,omitempty"`
	NodeID          uint         `gorm:"column:node_id;index;default:0" json:"nodeId"`
	Mode            string       `gorm:"size:20;not null;default:'quick'" json:"mode"`
	Status          string       `gorm:"size:20;index;not null" json:"status"`
	Summary         string       `gorm:"size:500" json:"summary"`
	ErrorMessage    string       `gorm:"column:error_message;size:2000" json:"errorMessage"`
	LogContent      string       `gorm:"column:log_content;type:text" json:"logContent"`
	DurationSeconds int          `gorm:"column:duration_seconds;not null;default:0" json:"durationSeconds"`
	StartedAt       time.Time    `gorm:"column:started_at;index;not null" json:"startedAt"`
	CompletedAt     *time.Time   `gorm:"column:completed_at;index" json:"completedAt,omitempty"`
	TriggeredBy     string       `gorm:"column:triggered_by;size:100" json:"triggeredBy"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

func (VerificationRecord) TableName() string {
	return "verification_records"
}
