package model

import "time"

// AgentInstallToken 一次性安装令牌，用于 /install/:token 公开端点。
//
// 生命周期：创建 → 消费（ConsumedAt 非空即作废）→ 超过 ExpiresAt 后被 GC 硬删除。
type AgentInstallToken struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Token       string     `gorm:"size:64;uniqueIndex;not null" json:"token"`
	NodeID      uint       `gorm:"not null;index" json:"nodeId"`
	Mode        string     `gorm:"size:16;not null" json:"mode"`        // systemd|docker|foreground
	Arch        string     `gorm:"size:16;not null" json:"arch"`        // amd64|arm64|auto
	AgentVer    string     `gorm:"size:32;not null" json:"agentVersion"`
	DownloadSrc string     `gorm:"size:16;not null;default:'github'" json:"downloadSrc"`
	ExpiresAt   time.Time  `gorm:"not null;index" json:"expiresAt"`
	ConsumedAt  *time.Time `json:"consumedAt,omitempty"`
	CreatedByID uint       `gorm:"not null" json:"createdById"`
	CreatedAt   time.Time  `json:"createdAt"`
}

func (AgentInstallToken) TableName() string { return "agent_install_tokens" }

// 合法模式/架构/下载源常量
const (
	InstallModeSystemd    = "systemd"
	InstallModeDocker     = "docker"
	InstallModeForeground = "foreground"

	InstallArchAmd64 = "amd64"
	InstallArchArm64 = "arm64"
	InstallArchAuto  = "auto"

	InstallSourceGitHub  = "github"
	InstallSourceGhproxy = "ghproxy"
)
