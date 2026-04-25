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
	ID           uint   `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string `gorm:"column:password_hash;not null" json:"-"`
	DisplayName  string `gorm:"size:128;not null" json:"displayName"`
	Email        string `gorm:"size:255" json:"email"`
	Phone        string `gorm:"size:64" json:"phone"`
	Role         string `gorm:"size:32;not null;default:admin" json:"role"`
	// TwoFactorSecretCiphertext 保存 TOTP 密钥密文；未启用时可作为待确认密钥。
	TwoFactorEnabled          bool   `gorm:"column:two_factor_enabled;not null;default:false" json:"twoFactorEnabled"`
	TwoFactorSecretCiphertext string `gorm:"column:two_factor_secret_ciphertext;type:text" json:"-"`
	// TwoFactorRecoveryCodeHashes 保存一次性恢复码哈希的 JSON 数组。
	TwoFactorRecoveryCodeHashes string `gorm:"column:two_factor_recovery_code_hashes;type:text" json:"-"`
	// WebAuthnCredentials 保存通行密钥公钥元数据 JSON，不包含私钥或明文密钥。
	WebAuthnCredentials         string `gorm:"column:webauthn_credentials;type:text" json:"-"`
	WebAuthnChallengeCiphertext string `gorm:"column:webauthn_challenge_ciphertext;type:text" json:"-"`
	TrustedDevices              string `gorm:"column:trusted_devices;type:text" json:"-"`
	EmailOTPEnabled             bool   `gorm:"column:email_otp_enabled;not null;default:false" json:"emailOtpEnabled"`
	SMSOTPEnabled               bool   `gorm:"column:sms_otp_enabled;not null;default:false" json:"smsOtpEnabled"`
	OutOfBandOTPCiphertext      string `gorm:"column:out_of_band_otp_ciphertext;type:text" json:"-"`
	// Disabled 禁用账号（不删除保留审计）。禁用后无法登录。
	Disabled  bool      `gorm:"not null;default:false" json:"disabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (User) TableName() string {
	return "users"
}
