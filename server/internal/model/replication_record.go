package model

import "time"

// ReplicationRecord 记录一次备份复制的执行。
// 触发方式：
//   - 自动：备份成功后，根据 task.ReplicationTargetIDs 自动派发
//   - 手动：从备份记录详情页手动触发
//
// 核心语义：把源存储上的备份对象 mirror 到目标存储，保留 StoragePath。
// 3-2-1 规则核心：每份备份至少存在于两个独立存储目标，且至少一份异地。
const (
	ReplicationStatusRunning = "running"
	ReplicationStatusSuccess = "success"
	ReplicationStatusFailed  = "failed"
)

type ReplicationRecord struct {
	ID             uint         `gorm:"primaryKey" json:"id"`
	BackupRecordID uint         `gorm:"column:backup_record_id;index;not null" json:"backupRecordId"`
	BackupRecord   BackupRecord `json:"backupRecord,omitempty"`
	TaskID         uint         `gorm:"column:task_id;index;not null" json:"taskId"`
	// SourceTargetID 源存储目标（备份已存在于此）
	SourceTargetID uint          `gorm:"column:source_target_id;index;not null" json:"sourceTargetId"`
	SourceTarget   StorageTarget `gorm:"foreignKey:SourceTargetID;references:ID" json:"sourceTarget,omitempty"`
	// DestTargetID 目标存储（复制过去）
	DestTargetID uint          `gorm:"column:dest_target_id;index;not null" json:"destTargetId"`
	DestTarget   StorageTarget `gorm:"foreignKey:DestTargetID;references:ID" json:"destTarget,omitempty"`
	Status       string        `gorm:"size:20;index;not null" json:"status"`
	StoragePath  string        `gorm:"column:storage_path;size:500" json:"storagePath"`
	FileSize     int64         `gorm:"column:file_size;not null;default:0" json:"fileSize"`
	Checksum     string        `gorm:"column:checksum;size:64" json:"checksum"`
	ErrorMessage string        `gorm:"column:error_message;size:2000" json:"errorMessage"`
	DurationSeconds int        `gorm:"column:duration_seconds;not null;default:0" json:"durationSeconds"`
	TriggeredBy  string        `gorm:"column:triggered_by;size:100" json:"triggeredBy"`
	StartedAt    time.Time     `gorm:"column:started_at;index;not null" json:"startedAt"`
	CompletedAt  *time.Time    `gorm:"column:completed_at;index" json:"completedAt,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

func (ReplicationRecord) TableName() string {
	return "replication_records"
}
