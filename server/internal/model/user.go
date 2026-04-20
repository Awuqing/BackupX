package model

import "time"

// 用户角色常量。RBAC 策略：
//   - admin：系统全权（创建用户、管理 API Key、删除数据、改设置）
//   - operator：日常运维（创建/编辑/执行任务、触发恢复与验证、管理存储目标与通知）
//   - viewer：只读（查看仪表盘、任务、记录、日志，不能触发或改变状态）
const (
	UserRoleAdmin    = "admin"
	UserRoleOperator = "operator"
	UserRoleViewer   = "viewer"
)

// IsValidRole 校验角色字符串合法。
func IsValidRole(role string) bool {
	switch role {
	case UserRoleAdmin, UserRoleOperator, UserRoleViewer:
		return true
	}
	return false
}

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"column:password_hash;not null" json:"-"`
	DisplayName  string    `gorm:"size:128;not null" json:"displayName"`
	Email        string    `gorm:"size:255" json:"email"`
	Role         string    `gorm:"size:32;not null;default:admin" json:"role"`
	// Disabled 禁用账号（不删除保留审计）。禁用后无法登录。
	Disabled  bool      `gorm:"not null;default:false" json:"disabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (User) TableName() string {
	return "users"
}
