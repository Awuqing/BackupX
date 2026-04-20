package model

import "time"

// AgentCommand 状态常量
const (
	AgentCommandStatusPending   = "pending"    // 待 Agent 拉取
	AgentCommandStatusDispatched = "dispatched" // Agent 已领取，正在执行
	AgentCommandStatusSucceeded = "succeeded"  // 执行成功
	AgentCommandStatusFailed    = "failed"     // 执行失败
	AgentCommandStatusTimeout   = "timeout"    // 超时未完成
)

// AgentCommand 类型常量
const (
	// AgentCommandTypeRunTask 运行指定备份任务
	// Payload: {"taskId": 123, "recordId": 456}
	AgentCommandTypeRunTask = "run_task"
	// AgentCommandTypeListDir 远程列目录（用于文件备份时的目录浏览器）
	// Payload: {"path": "/var/log"}
	// Result:  {"entries": [{"name":"...", "path":"...", "isDir":true, "size":0}]}
	AgentCommandTypeListDir = "list_dir"
	// AgentCommandTypeRestoreRecord 在 Agent 节点上恢复指定备份记录
	// Payload: {"restoreRecordId": 789}
	// Agent 拉 /api/agent/restores/:id/spec 获取完整规格后执行恢复
	AgentCommandTypeRestoreRecord = "restore_record"
	// AgentCommandTypeDiscoverDB 在 Agent 节点上发现数据库列表
	// Payload: {"type": "mysql", "host": "...", "port": 3306, "user": "...", "password": "..."}
	// Result:  {"databases": ["db1", "db2"]}
	AgentCommandTypeDiscoverDB = "discover_db"
	// AgentCommandTypeDeleteStorageObject 在 Agent 节点上删除指定存储对象
	// Payload: {"targetType": "local_disk", "targetConfig": {...}, "storagePath": "tasks/1/x.tar.gz"}
	// 用于跨节点 local_disk 场景：Master 删记录时请求 Agent 清理其本地备份文件。
	// Agent 需具备对应存储 provider 的执行能力。best-effort：失败仅影响 Agent 侧文件残留。
	AgentCommandTypeDeleteStorageObject = "delete_storage_object"
)

// AgentCommand 代表 Master 发给某个 Agent 节点的待执行命令。
// 使用简单的数据库队列实现：Agent 通过 token 长轮询拉取本节点 pending 命令，
// 执行后回写状态与结果。Master 侧通过定时检查把超时的命令标记为 timeout。
type AgentCommand struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	NodeID         uint       `gorm:"column:node_id;index;not null" json:"nodeId"`
	Type           string     `gorm:"size:32;index;not null" json:"type"`
	Status         string     `gorm:"size:20;index;not null;default:'pending'" json:"status"`
	Payload        string     `gorm:"type:text" json:"payload"`        // JSON
	Result         string     `gorm:"type:text" json:"result"`         // JSON（成功结果）
	ErrorMessage   string     `gorm:"column:error_message;type:text" json:"errorMessage"`
	DispatchedAt   *time.Time `gorm:"column:dispatched_at" json:"dispatchedAt,omitempty"`
	CompletedAt    *time.Time `gorm:"column:completed_at" json:"completedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (AgentCommand) TableName() string {
	return "agent_commands"
}
