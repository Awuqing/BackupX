package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/security"
)

type OTPConfigInput struct {
	CurrentPassword string `json:"currentPassword" binding:"required,min=8,max=128"`
	Channel         string `json:"channel" binding:"required,oneof=email sms"`
	Enabled         bool   `json:"enabled"`
	Email           string `json:"email" binding:"omitempty,max=255"`
	Phone           string `json:"phone" binding:"omitempty,max=64"`
}

type LoginOTPInput struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=8,max=128"`
	Channel  string `json:"channel" binding:"required,oneof=email sms"`
}

func (s *AuthService) ConfigureOutOfBandOTP(ctx context.Context, subject string, input OTPConfigInput) (*UserOutput, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	if err := security.ComparePassword(user.PasswordHash, input.CurrentPassword); err != nil {
		return nil, apperror.BadRequest("AUTH_WRONG_PASSWORD", "当前密码不正确", err)
	}
	channel := strings.TrimSpace(input.Channel)
	previousEmail := strings.TrimSpace(user.Email)
	previousPhone := strings.TrimSpace(user.Phone)
	contactChanged := false
	switch channel {
	case "email":
		email := strings.TrimSpace(input.Email)
		if email != "" {
			user.Email = email
		}
		contactChanged = previousEmail != strings.TrimSpace(user.Email)
		if input.Enabled && strings.TrimSpace(user.Email) == "" {
			return nil, apperror.BadRequest("AUTH_EMAIL_REQUIRED", "请先在用户资料中设置邮箱", nil)
		}
		user.EmailOTPEnabled = input.Enabled
	case "sms":
		phone := strings.TrimSpace(input.Phone)
		if phone != "" {
			user.Phone = phone
		}
		contactChanged = previousPhone != strings.TrimSpace(user.Phone)
		if input.Enabled && strings.TrimSpace(user.Phone) == "" {
			return nil, apperror.BadRequest("AUTH_PHONE_REQUIRED", "请先设置手机号", nil)
		}
		user.SMSOTPEnabled = input.Enabled
	default:
		return nil, apperror.BadRequest("AUTH_OTP_CHANNEL_INVALID", "验证码渠道不支持", nil)
	}
	if s.shouldClearPendingOTP(user, channel, contactChanged) {
		user.OutOfBandOTPCiphertext = ""
	}
	clearTrustedDevicesIfMFAOff(user)
	if err := s.users.Update(ctx, user); err != nil {
		return nil, apperror.Internal("AUTH_OTP_CONFIG_FAILED", "无法更新 OTP 配置", err)
	}
	if s.auditService != nil {
		action := "otp_disable"
		if input.Enabled {
			action = "otp_enable"
		}
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: action,
			TargetType: "otp", TargetID: channel,
			Detail: fmt.Sprintf("%s %s OTP", map[bool]string{true: "启用", false: "关闭"}[input.Enabled], channel),
		})
	}
	return ToUserOutput(user), nil
}

func (s *AuthService) SendLoginOTP(ctx context.Context, input LoginOTPInput, clientKey string) error {
	user, err := s.verifyPasswordForMFAStart(ctx, input.Username, input.Password, clientKey)
	if err != nil {
		return err
	}
	channel := strings.TrimSpace(input.Channel)
	if channel == "email" && !user.EmailOTPEnabled {
		return apperror.BadRequest("AUTH_EMAIL_OTP_DISABLED", "当前账号未启用邮件验证码", nil)
	}
	if channel == "sms" && !user.SMSOTPEnabled {
		return apperror.BadRequest("AUTH_SMS_OTP_DISABLED", "当前账号未启用短信验证码", nil)
	}
	code, err := security.GenerateNumericOTP()
	if err != nil {
		return apperror.Internal("AUTH_OTP_GENERATE_FAILED", "无法生成登录验证码", err)
	}
	hash, err := security.HashPassword(code)
	if err != nil {
		return apperror.Internal("AUTH_OTP_GENERATE_FAILED", "无法处理登录验证码", err)
	}
	pending := pendingOutOfBandOTP{
		Channel:   channel,
		CodeHash:  hash,
		ExpiresAt: time.Now().UTC().Add(mfaChallengeTTL),
	}
	ciphertext, err := s.twoFactorCipher.EncryptJSON(pending)
	if err != nil {
		return apperror.Internal("AUTH_OTP_SAVE_FAILED", "无法保存登录验证码状态", err)
	}
	user.OutOfBandOTPCiphertext = ciphertext
	if err := s.users.Update(ctx, user); err != nil {
		return apperror.Internal("AUTH_OTP_SAVE_FAILED", "无法保存登录验证码状态", err)
	}
	if err := s.deliverLoginOTP(ctx, user, channel, code); err != nil {
		user.OutOfBandOTPCiphertext = ""
		if updateErr := s.users.Update(ctx, user); updateErr != nil {
			return apperror.Internal("AUTH_OTP_SAVE_FAILED", "登录验证码发送失败，且无法回滚验证码状态", updateErr)
		}
		return apperror.BadRequest("AUTH_OTP_DELIVERY_FAILED", "登录验证码发送失败", err)
	}
	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "otp_send",
			TargetType: "otp", TargetID: channel,
			Detail: "发送登录 OTP", ClientIP: clientKey,
		})
	}
	return nil
}

func (s *AuthService) consumeOutOfBandOTP(ctx context.Context, user *model.User, code string, clientKey string) (bool, error) {
	if strings.TrimSpace(user.OutOfBandOTPCiphertext) == "" {
		return false, nil
	}
	var pending pendingOutOfBandOTP
	if err := s.twoFactorCipher.DecryptJSON(user.OutOfBandOTPCiphertext, &pending); err != nil {
		return false, apperror.Internal("AUTH_OTP_INVALID", "登录验证码状态异常", err)
	}
	if pending.ExpiresAt.Before(time.Now().UTC()) {
		user.OutOfBandOTPCiphertext = ""
		if err := s.users.Update(ctx, user); err != nil {
			return false, apperror.Internal("AUTH_OTP_CONSUME_FAILED", "无法更新登录验证码状态", err)
		}
		return false, nil
	}
	if !outOfBandOTPChannelEnabled(user, pending.Channel) {
		user.OutOfBandOTPCiphertext = ""
		if err := s.users.Update(ctx, user); err != nil {
			return false, apperror.Internal("AUTH_OTP_CONSUME_FAILED", "无法更新登录验证码状态", err)
		}
		return false, nil
	}
	if security.ComparePassword(pending.CodeHash, security.NormalizeNumericOTP(code)) != nil {
		return false, nil
	}
	user.OutOfBandOTPCiphertext = ""
	if err := s.users.Update(ctx, user); err != nil {
		return false, apperror.Internal("AUTH_OTP_CONSUME_FAILED", "无法使用登录验证码", err)
	}
	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "otp_used",
			TargetType: "otp", TargetID: pending.Channel,
			Detail: "使用登录 OTP 完成登录", ClientIP: clientKey,
		})
	}
	return true, nil
}

func (s *AuthService) deliverLoginOTP(ctx context.Context, user *model.User, channel string, code string) error {
	if s.notificationService == nil {
		return fmt.Errorf("notification service is not configured")
	}
	switch channel {
	case "email":
		email := strings.TrimSpace(user.Email)
		if email == "" {
			return fmt.Errorf("user email is empty")
		}
		return s.notificationService.SendAuthEmailOTP(ctx, email, code)
	case "sms":
		phone := strings.TrimSpace(user.Phone)
		if phone == "" {
			return fmt.Errorf("user phone is empty")
		}
		return s.notificationService.SendAuthSMSOTP(ctx, phone, code)
	default:
		return fmt.Errorf("unsupported otp channel: %s", channel)
	}
}

func (s *AuthService) verifyPasswordForMFAStart(ctx context.Context, username string, password string, clientKey string) (*model.User, error) {
	if clientKey == "" {
		clientKey = "unknown"
	}
	if !s.rateLimiter.Allow(clientKey) {
		return nil, apperror.TooManyRequests("AUTH_RATE_LIMITED", "登录尝试过于频繁，请稍后再试", nil)
	}
	user, err := s.users.FindByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		return nil, apperror.Internal("AUTH_LOOKUP_FAILED", "无法执行登录校验", err)
	}
	if user == nil || user.Disabled {
		return nil, apperror.Unauthorized("AUTH_INVALID_CREDENTIALS", "用户名或密码错误", nil)
	}
	if err := security.ComparePassword(user.PasswordHash, password); err != nil {
		if s.auditService != nil {
			s.auditService.Record(AuditEntry{
				UserID: user.ID, Username: user.Username,
				Category: "auth", Action: "login_failed",
				Detail: "密码错误", ClientIP: clientKey,
			})
		}
		return nil, apperror.Unauthorized("AUTH_INVALID_CREDENTIALS", "用户名或密码错误", err)
	}
	if !userMFAEnabled(user) {
		return nil, apperror.BadRequest("AUTH_MFA_NOT_ENABLED", "当前账号未启用多因素验证", nil)
	}
	return user, nil
}

func outOfBandOTPChannelEnabled(user *model.User, channel string) bool {
	switch channel {
	case "email":
		return user.EmailOTPEnabled
	case "sms":
		return user.SMSOTPEnabled
	default:
		return false
	}
}

func (s *AuthService) shouldClearPendingOTP(user *model.User, changedChannel string, contactChanged bool) bool {
	if !user.EmailOTPEnabled && !user.SMSOTPEnabled {
		return true
	}
	if strings.TrimSpace(user.OutOfBandOTPCiphertext) == "" {
		return false
	}
	var pending pendingOutOfBandOTP
	if err := s.twoFactorCipher.DecryptJSON(user.OutOfBandOTPCiphertext, &pending); err != nil {
		return true
	}
	return pending.Channel == changedChannel && (contactChanged || !outOfBandOTPChannelEnabled(user, changedChannel))
}
