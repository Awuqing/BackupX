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
