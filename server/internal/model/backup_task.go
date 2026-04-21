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
	// NodePoolTag 节点池标签（可选）。非空且 NodeID=0 时，调度器会从 Node.Labels 包含该 tag
	// 的在线节点中动态挑选一台执行（按运行中任务数最少原则），失败会 best-effort 切换到下一个候选。
	// 典型场景：NodePoolTag="db" 让 MySQL 备份任务在任意标有 "db" 的数据库节点执行。
	NodePoolTag string `gorm:"column:node_pool_tag;size:64;index" json:"nodePoolTag"`
	Tags                 string          `gorm:"column:tags;size:500" json:"tags"`
	RetentionDays        int             `gorm:"column:retention_days;not null;default:30" json:"retentionDays"`
	Compression          string          `gorm:"size:10;not null;default:'gzip'" json:"compression"`
	Encrypt              bool            `gorm:"not null;default:false" json:"encrypt"`
	MaxBackups           int             `gorm:"column:max_backups;not null;default:10" json:"maxBackups"`
	LastRunAt            *time.Time      `gorm:"column:last_run_at" json:"lastRunAt,omitempty"`
	LastStatus           string          `gorm:"column:last_status;size:20;not null;default:'idle'" json:"lastStatus"`
	// 验证（恢复演练）配置 — 定期自动校验备份可恢复性
	VerifyEnabled   bool   `gorm:"column:verify_enabled;not null;default:false" json:"verifyEnabled"`
	VerifyCronExpr  string `gorm:"column:verify_cron_expr;size:64" json:"verifyCronExpr"`
	VerifyMode      string `gorm:"column:verify_mode;size:20;not null;default:'quick'" json:"verifyMode"`
	// SLA 配置 — RPO（期望最长未备份间隔）与告警阈值
	SLAHoursRPO             int `gorm:"column:sla_hours_rpo;not null;default:0" json:"slaHoursRpo"`
	AlertOnConsecutiveFails int `gorm:"column:alert_on_consecutive_fails;not null;default:1" json:"alertOnConsecutiveFails"`
	// ReplicationTargetIDs 备份复制目标存储 ID 列表（CSV）。
	// 备份完成后，系统将自动把成果从任务主存储（StorageTargets 的第一个）复制到这些目标。
	// 满足 3-2-1 规则：至少 2 份副本，且至少 1 份异地（不同 provider/region）。
	ReplicationTargetIDs string `gorm:"column:replication_target_ids;size:500" json:"replicationTargetIds"`
	// MaintenanceWindows 允许执行备份的时段（格式详见 backup/window.go）。
	// 空 = 不限制。非空时调度器在非窗口跳过，手动执行返回友好错误。
	MaintenanceWindows string `gorm:"column:maintenance_windows;size:500" json:"maintenanceWindows"`
	// DependsOnTaskIDs 依赖的上游任务 ID 列表（CSV）。
	// 语义：上游任务成功后自动触发本任务，形成工作流（如 DB 备份完成 → 归档压缩）。
	// 调度器继续按本任务自己的 cron 触发，仅"自动触发"路径响应依赖完成事件。
	// 循环依赖检查在 service 层完成，避免配置阶段即出错。
	DependsOnTaskIDs string `gorm:"column:depends_on_task_ids;size:500" json:"dependsOnTaskIds"`
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
