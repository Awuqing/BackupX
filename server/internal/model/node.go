package model

import "time"

const (
	NodeStatusOnline  = "online"
	NodeStatusOffline = "offline"
)

// Node represents a managed server node in the cluster.
// The default "local" node is auto-created for single-machine backward compatibility.
type Node struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;uniqueIndex;not null" json:"name"`
	Hostname  string    `gorm:"size:255" json:"hostname"`
	IPAddress string    `gorm:"column:ip_address;size:64" json:"ipAddress"`
	Token     string    `gorm:"size:128;uniqueIndex;not null" json:"-"`
	Status    string    `gorm:"size:20;not null;default:'offline'" json:"status"`
	IsLocal   bool      `gorm:"not null;default:false" json:"isLocal"`
	OS        string    `gorm:"size:64" json:"os"`
	Arch      string    `gorm:"size:32" json:"arch"`
	AgentVer             string     `gorm:"column:agent_version;size:32" json:"agentVersion"`
	LastSeen             time.Time  `gorm:"column:last_seen" json:"lastSeen"`
	PrevToken            string     `gorm:"size:128;index" json:"-"`
	PrevTokenExpires     *time.Time `gorm:"column:prev_token_expires" json:"-"`
	// MaxConcurrent 该节点允许的最大并发任务数（0=不限制，沿用全局 cfg.Backup.MaxConcurrent）。
	// 用于大集群中限制单节点资源占用：例如小内存 Agent 节点可配 1，避免多个大备份同时跑挤爆。
	MaxConcurrent int `gorm:"column:max_concurrent;not null;default:0" json:"maxConcurrent"`
	// BandwidthLimit 该节点上传带宽上限（rclone 可识别格式：10M / 1G / 0=不限）。
	// 对集群感知的上传场景有效（Master 本地与 Agent 运行时均会应用）。
	BandwidthLimit string `gorm:"column:bandwidth_limit;size:32" json:"bandwidthLimit"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (Node) TableName() string {
	return "nodes"
}
