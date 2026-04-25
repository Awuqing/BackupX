package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
	"backupx/server/internal/storage/codec"
)

type SetupInput struct {
	Username    string `json:"username" binding:"required,min=3,max=64"`
	Password    string `json:"password" binding:"required,min=8,max=128"`
	DisplayName string `json:"displayName" binding:"required,min=1,max=128"`
}

type LoginInput struct {
	Username           string                           `json:"username" binding:"required,min=3,max=64"`
	Password           string                           `json:"password" binding:"required,min=8,max=128"`
	TwoFactorCode      string                           `json:"twoFactorCode" binding:"omitempty,min=6,max=32"`
	WebAuthnAssertion  *security.WebAuthnLoginAssertion `json:"webAuthnAssertion"`
	TrustedDeviceToken string                           `json:"trustedDeviceToken"`
	RememberDevice     bool                             `json:"rememberDevice"`
	TrustedDeviceName  string                           `json:"trustedDeviceName" binding:"omitempty,max=128"`
}

type AuthPayload struct {
	Token              string               `json:"token"`
	User               *UserOutput          `json:"user"`
	TrustedDeviceToken string               `json:"trustedDeviceToken,omitempty"`
	TrustedDevice      *TrustedDeviceOutput `json:"trustedDevice,omitempty"`
}

type UserOutput struct {
	ID                              uint   `json:"id"`
	Username                        string `json:"username"`
	DisplayName                     string `json:"displayName"`
	Email                           string `json:"email"`
	Phone                           string `json:"phone"`
	Role                            string `json:"role"`
	MFAEnabled                      bool   `json:"mfaEnabled"`
	TwoFactorEnabled                bool   `json:"twoFactorEnabled"`
	TwoFactorRecoveryCodesRemaining int    `json:"twoFactorRecoveryCodesRemaining"`
	WebAuthnEnabled                 bool   `json:"webAuthnEnabled"`
	WebAuthnCredentialCount         int    `json:"webAuthnCredentialCount"`
	TrustedDeviceCount              int    `json:"trustedDeviceCount"`
	EmailOTPEnabled                 bool   `json:"emailOtpEnabled"`
	SMSOTPEnabled                   bool   `json:"smsOtpEnabled"`
}

type AuthService struct {
	users               repository.UserRepository
	configs             repository.SystemConfigRepository
	jwtManager          *security.JWTManager
	rateLimiter         *security.LoginRateLimiter
	twoFactorCipher     *codec.ConfigCipher
	auditService        *AuditService
	notificationService *NotificationService
}

func NewAuthService(
	users repository.UserRepository,
	configs repository.SystemConfigRepository,
	jwtManager *security.JWTManager,
	rateLimiter *security.LoginRateLimiter,
	twoFactorCipher *codec.ConfigCipher,
) *AuthService {
	return &AuthService{
		users:           users,
		configs:         configs,
		jwtManager:      jwtManager,
		rateLimiter:     rateLimiter,
		twoFactorCipher: twoFactorCipher,
	}
}

func (s *AuthService) SetAuditService(auditService *AuditService) {
	s.auditService = auditService
}

func (s *AuthService) SetNotificationService(notificationService *NotificationService) {
	s.notificationService = notificationService
}

func (s *AuthService) SetupStatus(ctx context.Context) (bool, error) {
	count, err := s.users.Count(ctx)
	if err != nil {
		return false, apperror.Internal("AUTH_STATUS_FAILED", "无法检查初始化状态", err)
	}
	return count > 0, nil
}

func (s *AuthService) Setup(ctx context.Context, input SetupInput) (*AuthPayload, error) {
	initialized, err := s.SetupStatus(ctx)
	if err != nil {
		return nil, err
	}
	if initialized {
		return nil, apperror.Conflict("AUTH_SETUP_DISABLED", "系统已初始化，请直接登录", nil)
	}

	existing, err := s.users.FindByUsername(ctx, strings.TrimSpace(input.Username))
	if err != nil {
		return nil, apperror.Internal("AUTH_LOOKUP_FAILED", "无法检查账户状态", err)
	}
	if existing != nil {
		return nil, apperror.Conflict("AUTH_USERNAME_EXISTS", "用户名已存在", nil)
	}

	hash, err := security.HashPassword(input.Password)
	if err != nil {
		return nil, apperror.Internal("AUTH_HASH_FAILED", "无法处理密码", err)
	}

	user := &model.User{
		Username:     strings.TrimSpace(input.Username),
		PasswordHash: hash,
		DisplayName:  strings.TrimSpace(input.DisplayName),
		Role:         "admin",
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, apperror.Internal("AUTH_CREATE_USER_FAILED", "无法创建管理员账户", err)
	}

	token, err := s.jwtManager.Generate(user)
	if err != nil {
		return nil, apperror.Internal("AUTH_TOKEN_FAILED", "无法生成访问令牌", err)
	}

	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "setup",
			TargetType: "user", TargetID: fmt.Sprintf("%d", user.ID), TargetName: user.Username,
			Detail: "系统初始化，创建管理员账户",
		})
	}

	return &AuthPayload{Token: token, User: ToUserOutput(user)}, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput, clientKey string) (*AuthPayload, error) {
	if clientKey == "" {
		clientKey = "unknown"
	}
	if !s.rateLimiter.Allow(clientKey) {
		return nil, apperror.TooManyRequests("AUTH_RATE_LIMITED", "登录尝试过于频繁，请稍后再试", nil)
	}

	user, err := s.users.FindByUsername(ctx, strings.TrimSpace(input.Username))
	if err != nil {
		return nil, apperror.Internal("AUTH_LOOKUP_FAILED", "无法执行登录校验", err)
	}
	if user == nil {
		if s.auditService != nil {
			s.auditService.Record(AuditEntry{
				Category: "auth", Action: "login_failed",
				Detail:   fmt.Sprintf("用户名不存在: %s", strings.TrimSpace(input.Username)),
				ClientIP: clientKey,
			})
		}
		return nil, apperror.Unauthorized("AUTH_INVALID_CREDENTIALS", "用户名或密码错误", nil)
	}
	if user.Disabled {
		if s.auditService != nil {
			s.auditService.Record(AuditEntry{
				UserID: user.ID, Username: user.Username,
				Category: "auth", Action: "login_rejected",
				Detail: "账号已被停用", ClientIP: clientKey,
			})
		}
		return nil, apperror.Unauthorized("AUTH_USER_DISABLED", "账号已被管理员停用", nil)
	}
	if err := security.ComparePassword(user.PasswordHash, input.Password); err != nil {
		if s.auditService != nil {
			s.auditService.Record(AuditEntry{
				UserID: user.ID, Username: user.Username,
				Category: "auth", Action: "login_failed",
				Detail: "密码错误", ClientIP: clientKey,
			})
		}
		return nil, apperror.Unauthorized("AUTH_INVALID_CREDENTIALS", "用户名或密码错误", err)
	}
	mfaRequired := userMFAEnabled(user)
	trustedDeviceUsed := false
	if mfaRequired {
		trusted, err := s.verifyTrustedDevice(ctx, user, input.TrustedDeviceToken, clientKey)
		if err != nil {
			return nil, err
		}
		trustedDeviceUsed = trusted
		if !trusted {
			if err := s.verifyLoginMFA(ctx, user, input, clientKey); err != nil {
				return nil, err
			}
		}
	}

	s.rateLimiter.Reset(clientKey)
	token, err := s.jwtManager.Generate(user)
	if err != nil {
		return nil, apperror.Internal("AUTH_TOKEN_FAILED", "无法生成访问令牌", err)
	}

	payload := &AuthPayload{Token: token, User: ToUserOutput(user)}
	if mfaRequired && !trustedDeviceUsed && input.RememberDevice {
		deviceToken, device, err := s.issueTrustedDevice(ctx, user, input.TrustedDeviceName, clientKey)
		if err != nil {
			return nil, err
		}
		payload.TrustedDeviceToken = deviceToken
		payload.TrustedDevice = device
	}

	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "login_success",
			Detail: "登录成功", ClientIP: clientKey,
		})
	}

	return payload, nil
}

func (s *AuthService) verifyLoginMFA(ctx context.Context, user *model.User, input LoginInput, clientKey string) error {
	if input.WebAuthnAssertion != nil {
		if err := s.VerifyWebAuthnLogin(ctx, user, *input.WebAuthnAssertion, clientKey); err != nil {
			if s.auditService != nil {
				s.auditService.Record(AuditEntry{
					UserID: user.ID, Username: user.Username,
					Category: "auth", Action: "login_failed",
					Detail: "通行密钥校验失败", ClientIP: clientKey,
				})
			}
			return err
		}
		return nil
	}
	code := strings.TrimSpace(input.TwoFactorCode)
	if code == "" {
		if s.auditService != nil {
			s.auditService.Record(AuditEntry{
				UserID: user.ID, Username: user.Username,
				Category: "auth", Action: "two_factor_required",
				Detail: "登录需要多因素验证", ClientIP: clientKey,
			})
		}
		return apperror.Unauthorized("AUTH_2FA_REQUIRED", "请输入验证码、恢复码或使用通行密钥", nil)
	}
	if user.TwoFactorEnabled {
		secret, err := s.decryptTwoFactorSecret(user.TwoFactorSecretCiphertext)
		if err != nil {
			return apperror.Internal("AUTH_2FA_SECRET_INVALID", "TOTP 配置异常", err)
		}
		ok, err := security.ValidateTOTPCode(secret, code)
		if err == nil && ok {
			return nil
		}
		if consumed, err := s.consumeRecoveryCode(ctx, user, code); err != nil {
			return err
		} else if consumed {
			if s.auditService != nil {
				s.auditService.Record(AuditEntry{
					UserID: user.ID, Username: user.Username,
					Category: "auth", Action: "two_factor_recovery_code_used",
					Detail: "使用恢复码完成登录", ClientIP: clientKey,
				})
			}
			return nil
		}
	}
	if consumed, err := s.consumeOutOfBandOTP(ctx, user, code, clientKey); err != nil {
		return err
	} else if consumed {
		return nil
	}
	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "login_failed",
			Detail: "多因素验证码错误", ClientIP: clientKey,
		})
	}
	return apperror.Unauthorized("AUTH_2FA_INVALID", "验证码、恢复码或通行密钥错误", nil)
}

func (s *AuthService) userBySubject(ctx context.Context, subject string) (*model.User, error) {
	userID, err := strconv.ParseUint(subject, 10, 64)
	if err != nil {
		return nil, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效用户身份", err)
	}
	user, err := s.users.FindByID(ctx, uint(userID))
	if err != nil {
		return nil, apperror.Internal("AUTH_LOOKUP_FAILED", "无法获取当前用户", err)
	}
	if user == nil {
		return nil, apperror.Unauthorized("AUTH_USER_NOT_FOUND", "当前用户不存在", errors.New("user not found"))
	}
	return user, nil
}

func (s *AuthService) GetCurrentUser(ctx context.Context, subject string) (*UserOutput, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	return ToUserOutput(user), nil
}

type ChangePasswordInput struct {
	OldPassword string `json:"oldPassword" binding:"required,min=8,max=128"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=128"`
}

func (s *AuthService) ChangePassword(ctx context.Context, subject string, input ChangePasswordInput) error {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return err
	}
	if err := security.ComparePassword(user.PasswordHash, input.OldPassword); err != nil {
		return apperror.BadRequest("AUTH_WRONG_PASSWORD", "旧密码不正确", err)
	}
	hash, err := security.HashPassword(input.NewPassword)
	if err != nil {
		return apperror.Internal("AUTH_HASH_FAILED", "无法处理密码", err)
	}
	user.PasswordHash = hash
	user.TrustedDevices = ""
	user.OutOfBandOTPCiphertext = ""
	user.WebAuthnChallengeCiphertext = ""
	if err := s.users.Update(ctx, user); err != nil {
		return apperror.Internal("AUTH_UPDATE_FAILED", "密码修改失败", err)
	}

	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "change_password",
			Detail: "密码修改成功",
		})
	}

	return nil
}

type TwoFactorSetupInput struct {
	CurrentPassword string `json:"currentPassword" binding:"required,min=8,max=128"`
}

type TwoFactorSetupOutput struct {
	Secret             string `json:"secret"`
	OTPAuthURL         string `json:"otpAuthUrl"`
	QRCodeDataURL      string `json:"qrCodeDataUrl"`
	TwoFactorEnabled   bool   `json:"twoFactorEnabled"`
	TwoFactorConfirmed bool   `json:"twoFactorConfirmed"`
}

type EnableTwoFactorInput struct {
	Code string `json:"code" binding:"required,min=6,max=10"`
}

type EnableTwoFactorOutput struct {
	User          *UserOutput `json:"user"`
	RecoveryCodes []string    `json:"recoveryCodes"`
}

type DisableTwoFactorInput struct {
	CurrentPassword string `json:"currentPassword" binding:"required,min=8,max=128"`
	Code            string `json:"code" binding:"required,min=6,max=32"`
}

type RegenerateRecoveryCodesInput struct {
	CurrentPassword string `json:"currentPassword" binding:"required,min=8,max=128"`
	Code            string `json:"code" binding:"required,min=6,max=10"`
}

type RecoveryCodesOutput struct {
	User          *UserOutput `json:"user"`
	RecoveryCodes []string    `json:"recoveryCodes"`
}

func (s *AuthService) PrepareTwoFactor(ctx context.Context, subject string, input TwoFactorSetupInput) (*TwoFactorSetupOutput, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	if user.TwoFactorEnabled {
		return nil, apperror.Conflict("AUTH_2FA_ALREADY_ENABLED", "TOTP 已启用", nil)
	}
	if err := security.ComparePassword(user.PasswordHash, input.CurrentPassword); err != nil {
		return nil, apperror.BadRequest("AUTH_WRONG_PASSWORD", "当前密码不正确", err)
	}

	enrollment, err := security.GenerateTOTPEnrollment(user.Username)
	if err != nil {
		return nil, apperror.Internal("AUTH_2FA_SETUP_FAILED", "无法生成 TOTP 密钥", err)
	}
	ciphertext, err := s.encryptTwoFactorSecret(enrollment.Secret)
	if err != nil {
		return nil, apperror.Internal("AUTH_2FA_SAVE_FAILED", "无法保存 TOTP 密钥", err)
	}
	user.TwoFactorSecretCiphertext = ciphertext
	user.TwoFactorEnabled = false
	if err := s.users.Update(ctx, user); err != nil {
		return nil, apperror.Internal("AUTH_2FA_SAVE_FAILED", "无法保存 TOTP 密钥", err)
	}

	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "two_factor_setup",
			TargetType: "user", TargetID: fmt.Sprintf("%d", user.ID), TargetName: user.Username,
			Detail: "生成 TOTP 密钥",
		})
	}

	return &TwoFactorSetupOutput{
		Secret:             enrollment.Secret,
		OTPAuthURL:         enrollment.OTPAuthURL,
		QRCodeDataURL:      enrollment.QRCodeDataURL,
		TwoFactorEnabled:   false,
		TwoFactorConfirmed: false,
	}, nil
}

func (s *AuthService) EnableTwoFactor(ctx context.Context, subject string, input EnableTwoFactorInput) (*EnableTwoFactorOutput, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	if user.TwoFactorEnabled {
		return nil, apperror.Conflict("AUTH_2FA_ALREADY_ENABLED", "TOTP 已启用", nil)
	}
	if strings.TrimSpace(user.TwoFactorSecretCiphertext) == "" {
		return nil, apperror.BadRequest("AUTH_2FA_NOT_PREPARED", "请先生成 TOTP 密钥", nil)
	}
	secret, err := s.decryptTwoFactorSecret(user.TwoFactorSecretCiphertext)
	if err != nil {
		return nil, apperror.Internal("AUTH_2FA_SECRET_INVALID", "TOTP 配置异常", err)
	}
	ok, err := security.ValidateTOTPCode(secret, input.Code)
	if err != nil {
		return nil, apperror.BadRequest("AUTH_2FA_INVALID", "TOTP 验证码格式不正确", err)
	}
	if !ok {
		return nil, apperror.BadRequest("AUTH_2FA_INVALID", "TOTP 验证码错误", nil)
	}
	recoveryCodes, recoveryHashes, err := s.generateRecoveryCodeHashes()
	if err != nil {
		return nil, apperror.Internal("AUTH_2FA_RECOVERY_FAILED", "无法生成恢复码", err)
	}

	user.TwoFactorEnabled = true
	user.TwoFactorRecoveryCodeHashes = recoveryHashes
	if err := s.users.Update(ctx, user); err != nil {
		return nil, apperror.Internal("AUTH_2FA_ENABLE_FAILED", "无法启用 TOTP", err)
	}
	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "two_factor_enable",
			TargetType: "user", TargetID: fmt.Sprintf("%d", user.ID), TargetName: user.Username,
			Detail: "启用 TOTP",
		})
	}
	return &EnableTwoFactorOutput{User: ToUserOutput(user), RecoveryCodes: recoveryCodes}, nil
}

func (s *AuthService) DisableTwoFactor(ctx context.Context, subject string, input DisableTwoFactorInput) (*UserOutput, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	if !user.TwoFactorEnabled {
		return nil, apperror.BadRequest("AUTH_2FA_NOT_ENABLED", "TOTP 未启用", nil)
	}
	if err := security.ComparePassword(user.PasswordHash, input.CurrentPassword); err != nil {
		return nil, apperror.BadRequest("AUTH_WRONG_PASSWORD", "当前密码不正确", err)
	}
	secret, err := s.decryptTwoFactorSecret(user.TwoFactorSecretCiphertext)
	if err != nil {
		return nil, apperror.Internal("AUTH_2FA_SECRET_INVALID", "TOTP 配置异常", err)
	}
	ok, err := security.ValidateTOTPCode(secret, input.Code)
	if err != nil {
		return nil, apperror.BadRequest("AUTH_2FA_INVALID", "TOTP 验证码格式不正确", err)
	}
	if !ok {
		return nil, apperror.BadRequest("AUTH_2FA_INVALID", "TOTP 验证码错误", nil)
	}

	user.TwoFactorEnabled = false
	user.TwoFactorSecretCiphertext = ""
	user.TwoFactorRecoveryCodeHashes = ""
	clearTrustedDevicesIfMFAOff(user)
	if err := s.users.Update(ctx, user); err != nil {
		return nil, apperror.Internal("AUTH_2FA_DISABLE_FAILED", "无法关闭 TOTP", err)
	}
	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "two_factor_disable",
			TargetType: "user", TargetID: fmt.Sprintf("%d", user.ID), TargetName: user.Username,
			Detail: "关闭 TOTP",
		})
	}
	return ToUserOutput(user), nil
}

func (s *AuthService) verifyCurrentTOTP(user *model.User, code string) error {
	secret, err := s.decryptTwoFactorSecret(user.TwoFactorSecretCiphertext)
	if err != nil {
		return apperror.Internal("AUTH_2FA_SECRET_INVALID", "TOTP 配置异常", err)
	}
	ok, err := security.ValidateTOTPCode(secret, code)
	if err != nil {
		return apperror.BadRequest("AUTH_2FA_INVALID", "TOTP 验证码格式不正确", err)
	}
	if !ok {
		return apperror.BadRequest("AUTH_2FA_INVALID", "TOTP 验证码错误", nil)
	}
	return nil
}

func (s *AuthService) RegenerateRecoveryCodes(ctx context.Context, subject string, input RegenerateRecoveryCodesInput) (*RecoveryCodesOutput, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	if !user.TwoFactorEnabled {
		return nil, apperror.BadRequest("AUTH_2FA_NOT_ENABLED", "TOTP 未启用", nil)
	}
	if err := security.ComparePassword(user.PasswordHash, input.CurrentPassword); err != nil {
		return nil, apperror.BadRequest("AUTH_WRONG_PASSWORD", "当前密码不正确", err)
	}
	if err := s.verifyCurrentTOTP(user, input.Code); err != nil {
		return nil, err
	}
	recoveryCodes, recoveryHashes, err := s.generateRecoveryCodeHashes()
	if err != nil {
		return nil, apperror.Internal("AUTH_2FA_RECOVERY_FAILED", "无法生成恢复码", err)
	}
	user.TwoFactorRecoveryCodeHashes = recoveryHashes
	if err := s.users.Update(ctx, user); err != nil {
		return nil, apperror.Internal("AUTH_2FA_RECOVERY_FAILED", "无法更新恢复码", err)
	}
	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "two_factor_recovery_codes_regenerate",
			TargetType: "user", TargetID: fmt.Sprintf("%d", user.ID), TargetName: user.Username,
			Detail: "重新生成 TOTP 恢复码",
		})
	}
	return &RecoveryCodesOutput{User: ToUserOutput(user), RecoveryCodes: recoveryCodes}, nil
}

func (s *AuthService) generateRecoveryCodeHashes() ([]string, string, error) {
	codes, err := security.GenerateRecoveryCodes(security.RecoveryCodeCount)
	if err != nil {
		return nil, "", err
	}
	hashes := make([]string, 0, len(codes))
	for _, code := range codes {
		hash, err := security.HashPassword(security.NormalizeRecoveryCode(code))
		if err != nil {
			return nil, "", err
		}
		hashes = append(hashes, hash)
	}
	encoded, err := encodeRecoveryCodeHashes(hashes)
	if err != nil {
		return nil, "", err
	}
	return codes, encoded, nil
}

func (s *AuthService) consumeRecoveryCode(ctx context.Context, user *model.User, code string) (bool, error) {
	if !security.IsRecoveryCodeCandidate(code) {
		return false, nil
	}
	hashes, err := parseRecoveryCodeHashes(user.TwoFactorRecoveryCodeHashes)
	if err != nil {
		return false, apperror.Internal("AUTH_2FA_RECOVERY_INVALID", "恢复码配置异常", err)
	}
	if len(hashes) == 0 {
		return false, nil
	}
	normalized := security.NormalizeRecoveryCode(code)
	for i, hash := range hashes {
		if security.ComparePassword(hash, normalized) != nil {
			continue
		}
		hashes = append(hashes[:i], hashes[i+1:]...)
		encoded, err := encodeRecoveryCodeHashes(hashes)
		if err != nil {
			return false, apperror.Internal("AUTH_2FA_RECOVERY_INVALID", "恢复码配置异常", err)
		}
		user.TwoFactorRecoveryCodeHashes = encoded
		if err := s.users.Update(ctx, user); err != nil {
			return false, apperror.Internal("AUTH_2FA_RECOVERY_CONSUME_FAILED", "无法使用恢复码", err)
		}
		return true, nil
	}
	return false, nil
}

func (s *AuthService) encryptTwoFactorSecret(secret string) (string, error) {
	if s.twoFactorCipher == nil {
		return "", errors.New("two-factor cipher is not configured")
	}
	return s.twoFactorCipher.Encrypt([]byte(strings.TrimSpace(secret)))
}

func (s *AuthService) decryptTwoFactorSecret(ciphertext string) (string, error) {
	if s.twoFactorCipher == nil {
		return "", errors.New("two-factor cipher is not configured")
	}
	raw, err := s.twoFactorCipher.Decrypt(strings.TrimSpace(ciphertext))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

func parseRecoveryCodeHashes(encoded string) ([]string, error) {
	if strings.TrimSpace(encoded) == "" {
		return nil, nil
	}
	var hashes []string
	if err := json.Unmarshal([]byte(encoded), &hashes); err != nil {
		return nil, err
	}
	return hashes, nil
}

func encodeRecoveryCodeHashes(hashes []string) (string, error) {
	if len(hashes) == 0 {
		return "", nil
	}
	encoded, err := json.Marshal(hashes)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func recoveryCodeRemainingCount(user *model.User) int {
	if user == nil {
		return 0
	}
	hashes, err := parseRecoveryCodeHashes(user.TwoFactorRecoveryCodeHashes)
	if err != nil {
		return 0
	}
	return len(hashes)
}

func ToUserOutput(user *model.User) *UserOutput {
	if user == nil {
		return nil
	}
	return &UserOutput{
		ID:                              user.ID,
		Username:                        user.Username,
		DisplayName:                     user.DisplayName,
		Email:                           user.Email,
		Phone:                           user.Phone,
		Role:                            user.Role,
		MFAEnabled:                      userMFAEnabled(user),
		TwoFactorEnabled:                user.TwoFactorEnabled,
		TwoFactorRecoveryCodesRemaining: recoveryCodeRemainingCount(user),
		WebAuthnEnabled:                 webAuthnCredentialCount(user) > 0,
		WebAuthnCredentialCount:         webAuthnCredentialCount(user),
		TrustedDeviceCount:              trustedDeviceCount(user),
		EmailOTPEnabled:                 user.EmailOTPEnabled,
		SMSOTPEnabled:                   user.SMSOTPEnabled,
	}
}

func SubjectFromContextValue(value any) (string, error) {
	subject, ok := value.(string)
	if !ok || strings.TrimSpace(subject) == "" {
		return "", fmt.Errorf("invalid subject context")
	}
	return subject, nil
}
