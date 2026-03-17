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
	AgentVer  string    `gorm:"column:agent_version;size:32" json:"agentVersion"`
	LastSeen  time.Time `gorm:"column:last_seen" json:"lastSeen"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Node) TableName() string {
	return "nodes"
}
