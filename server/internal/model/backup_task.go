package model

import "time"

const (
	BackupTaskTypeFile       = "file"
	BackupTaskTypeMySQL      = "mysql"
	BackupTaskTypeSQLite     = "sqlite"
	BackupTaskTypePostgreSQL = "postgresql"
	BackupTaskTypeSAPHANA    = "saphana"
)

const (
	BackupTaskStatusIdle    = "idle"
	BackupTaskStatusRunning = "running"
	BackupTaskStatusSuccess = "success"
	BackupTaskStatusFailed  = "failed"
)

type BackupTask struct {
	ID                   uint            `gorm:"primaryKey" json:"id"`
	Name                 string          `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Type                 string          `gorm:"size:20;index;not null" json:"type"`
	Enabled              bool            `gorm:"not null;default:true" json:"enabled"`
	CronExpr             string          `gorm:"column:cron_expr;size:64" json:"cronExpr"`
	SourcePath           string          `gorm:"column:source_path;size:500" json:"sourcePath"`
	SourcePaths          string          `gorm:"column:source_paths;type:text" json:"sourcePaths"`
	ExcludePatterns      string          `gorm:"column:exclude_patterns;type:text" json:"excludePatterns"`
	DBHost               string          `gorm:"column:db_host;size:255" json:"dbHost"`
	DBPort               int             `gorm:"column:db_port" json:"dbPort"`
	DBUser               string          `gorm:"column:db_user;size:100" json:"dbUser"`
	DBPasswordCiphertext string          `gorm:"column:db_password_ciphertext;type:text" json:"-"`
	DBName               string          `gorm:"column:db_name;size:255" json:"dbName"`
	DBPath               string          `gorm:"column:db_path;size:500" json:"dbPath"`
	// ExtraConfig 类型特有的扩展配置（JSON），如 SAP HANA 的 backupLevel / backupChannels 等
	ExtraConfig string `gorm:"column:extra_config;type:text" json:"extraConfig"`
	StorageTargetID      uint            `gorm:"column:storage_target_id;index;not null" json:"storageTargetId"`           // deprecated: 保留兼容
	StorageTarget        StorageTarget   `json:"storageTarget,omitempty"`                                                  // deprecated: 保留兼容
	StorageTargets       []StorageTarget `gorm:"many2many:backup_task_storage_targets" json:"storageTargets,omitempty"`
	NodeID               uint            `gorm:"column:node_id;index;default:0" json:"nodeId"`
	Node                 Node            `json:"node,omitempty"`
	Tags                 string          `gorm:"column:tags;size:500" json:"tags"`
	RetentionDays        int             `gorm:"column:retention_days;not null;default:30" json:"retentionDays"`
	Compression          string          `gorm:"size:10;not null;default:'gzip'" json:"compression"`
	Encrypt              bool            `gorm:"not null;default:false" json:"encrypt"`
	MaxBackups           int             `gorm:"column:max_backups;not null;default:10" json:"maxBackups"`
	LastRunAt            *time.Time      `gorm:"column:last_run_at" json:"lastRunAt,omitempty"`
	LastStatus           string          `gorm:"column:last_status;size:20;not null;default:'idle'" json:"lastStatus"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
}

func (BackupTask) TableName() string {
	return "backup_tasks"
}

// BackupTaskStorageTarget 多对多中间表
type BackupTaskStorageTarget struct {
	BackupTaskID    uint `gorm:"primaryKey;column:backup_task_id"`
	StorageTargetID uint `gorm:"primaryKey;column:storage_target_id"`
}

func (BackupTaskStorageTarget) TableName() string {
	return "backup_task_storage_targets"
}
