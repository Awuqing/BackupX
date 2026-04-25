package service

import (
	"context"
	"testing"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/security"
	"backupx/server/internal/storage/codec"
	"github.com/pquerna/otp/totp"
)

type fakeUserRepository struct {
	users []*model.User
}

func (r *fakeUserRepository) Count(context.Context) (int64, error) {
	return int64(len(r.users)), nil
}

func (r *fakeUserRepository) Create(_ context.Context, user *model.User) error {
	user.ID = uint(len(r.users) + 1)
	r.users = append(r.users, user)
	return nil
}

func (r *fakeUserRepository) FindByUsername(_ context.Context, username string) (*model.User, error) {
	for _, user := range r.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, nil
}

func (r *fakeUserRepository) FindByID(_ context.Context, id uint) (*model.User, error) {
	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, nil
}

func (r *fakeUserRepository) Update(_ context.Context, user *model.User) error {
	for i, u := range r.users {
		if u.ID == user.ID {
			r.users[i] = user
			return nil
		}
	}
	return nil
}

func (r *fakeUserRepository) CountByRole(_ context.Context, role string) (int64, error) {
	var n int64
	for _, u := range r.users {
		if u.Role == role && !u.Disabled {
			n++
		}
	}
	return n, nil
}

func (r *fakeUserRepository) List(_ context.Context) ([]model.User, error) {
	result := make([]model.User, 0, len(r.users))
	for _, u := range r.users {
		result = append(result, *u)
	}
	return result, nil
}

func (r *fakeUserRepository) Delete(_ context.Context, id uint) error {
	for i, u := range r.users {
		if u.ID == id {
			r.users = append(r.users[:i], r.users[i+1:]...)
			return nil
		}
	}
	return nil
}

type fakeSystemConfigRepository struct{}

func (r *fakeSystemConfigRepository) GetByKey(context.Context, string) (*model.SystemConfig, error) {
	return nil, nil
}

func (r *fakeSystemConfigRepository) List(context.Context) ([]model.SystemConfig, error) {
	return nil, nil
}

func (r *fakeSystemConfigRepository) Upsert(context.Context, *model.SystemConfig) error {
	return nil
}

func TestAuthServiceSetupAndLogin(t *testing.T) {
	users := &fakeUserRepository{}
	service := NewAuthService(
		users,
		&fakeSystemConfigRepository{},
		security.NewJWTManager("test-secret", time.Hour),
		security.NewLoginRateLimiter(5, time.Minute),
		codec.NewConfigCipher("test-encryption-secret"),
	)

	setupResult, err := service.Setup(context.Background(), SetupInput{
		Username:    "admin",
		Password:    "password-123",
		DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}
	if setupResult.User.Username != "admin" {
		t.Fatalf("expected username admin, got %s", setupResult.User.Username)
	}

	loginResult, err := service.Login(context.Background(), LoginInput{
		Username: "admin",
		Password: "password-123",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if loginResult.Token == "" {
		t.Fatalf("expected non-empty token")
	}
}

func newTestAuthService() (*AuthService, *fakeUserRepository) {
	users := &fakeUserRepository{}
	svc := NewAuthService(
		users,
		&fakeSystemConfigRepository{},
		security.NewJWTManager("test-secret", time.Hour),
		security.NewLoginRateLimiter(5, time.Minute),
		codec.NewConfigCipher("test-encryption-secret"),
	)
	return svc, users
}

func TestChangePassword(t *testing.T) {
	svc, _ := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	err = svc.ChangePassword(context.Background(), "1", ChangePasswordInput{
		OldPassword: "password-123",
		NewPassword: "new-password-456",
	})
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	// Old password should no longer work
	_, err = svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123",
	}, "127.0.0.1")
	if err == nil {
		t.Fatalf("expected login with old password to fail")
	}

	// New password should work
	_, err = svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "new-password-456",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}

func TestChangePasswordWrongOld(t *testing.T) {
	svc, _ := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	err = svc.ChangePassword(context.Background(), "1", ChangePasswordInput{
		OldPassword: "wrong-password",
		NewPassword: "new-password-456",
	})
	if err == nil {
		t.Fatalf("expected ChangePassword with wrong old password to fail")
	}
}

func TestAuthServiceLoginRequiresTwoFactorWhenEnabled(t *testing.T) {
	svc, _ := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	setup, err := svc.PrepareTwoFactor(context.Background(), "1", TwoFactorSetupInput{
		CurrentPassword: "password-123",
	})
	if err != nil {
		t.Fatalf("PrepareTwoFactor: %v", err)
	}
	if setup.Secret == "" || setup.QRCodeDataURL == "" || setup.OTPAuthURL == "" {
		t.Fatalf("expected populated 2FA enrollment, got %#v", setup)
	}

	code, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	enabledUser, err := svc.EnableTwoFactor(context.Background(), "1", EnableTwoFactorInput{Code: code})
	if err != nil {
		t.Fatalf("EnableTwoFactor: %v", err)
	}
	if !enabledUser.User.TwoFactorEnabled {
		t.Fatalf("expected 2FA enabled")
	}
	if len(enabledUser.RecoveryCodes) != security.RecoveryCodeCount {
		t.Fatalf("expected %d recovery codes, got %d", security.RecoveryCodeCount, len(enabledUser.RecoveryCodes))
	}

	_, err = svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123",
	}, "127.0.0.1")
	if appErr, ok := err.(*apperror.AppError); !ok || appErr.Code != "AUTH_2FA_REQUIRED" {
		t.Fatalf("expected AUTH_2FA_REQUIRED, got %v", err)
	}

	loginCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode login: %v", err)
	}
	loginResult, err := svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123", TwoFactorCode: loginCode,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Login with 2FA: %v", err)
	}
	if loginResult.Token == "" {
		t.Fatalf("expected non-empty token")
	}
}

func TestAuthServiceDisableTwoFactor(t *testing.T) {
	svc, _ := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	setup, err := svc.PrepareTwoFactor(context.Background(), "1", TwoFactorSetupInput{
		CurrentPassword: "password-123",
	})
	if err != nil {
		t.Fatalf("PrepareTwoFactor: %v", err)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := svc.EnableTwoFactor(context.Background(), "1", EnableTwoFactorInput{Code: code}); err != nil {
		t.Fatalf("EnableTwoFactor: %v", err)
	}

	disableCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode disable: %v", err)
	}
	user, err := svc.DisableTwoFactor(context.Background(), "1", DisableTwoFactorInput{
		CurrentPassword: "password-123",
		Code:            disableCode,
	})
	if err != nil {
		t.Fatalf("DisableTwoFactor: %v", err)
	}
	if user.TwoFactorEnabled {
		t.Fatalf("expected 2FA disabled")
	}

	loginResult, err := svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Login after disable: %v", err)
	}
	if loginResult.Token == "" {
		t.Fatalf("expected non-empty token")
	}
}

func TestAuthServiceRecoveryCodeLoginConsumesCode(t *testing.T) {
	svc, _ := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	setup, err := svc.PrepareTwoFactor(context.Background(), "1", TwoFactorSetupInput{
		CurrentPassword: "password-123",
	})
	if err != nil {
		t.Fatalf("PrepareTwoFactor: %v", err)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	enabled, err := svc.EnableTwoFactor(context.Background(), "1", EnableTwoFactorInput{Code: code})
	if err != nil {
		t.Fatalf("EnableTwoFactor: %v", err)
	}
	recoveryCode := enabled.RecoveryCodes[0]

	loginResult, err := svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123", TwoFactorCode: recoveryCode,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Login with recovery code: %v", err)
	}
	if loginResult.User.TwoFactorRecoveryCodesRemaining != security.RecoveryCodeCount-1 {
		t.Fatalf("expected one recovery code consumed, got remaining=%d", loginResult.User.TwoFactorRecoveryCodesRemaining)
	}

	_, err = svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123", TwoFactorCode: recoveryCode,
	}, "127.0.0.1")
	if appErr, ok := err.(*apperror.AppError); !ok || appErr.Code != "AUTH_2FA_INVALID" {
		t.Fatalf("expected consumed recovery code to fail, got %v", err)
	}
}

func TestAuthServiceRegenerateRecoveryCodesInvalidatesOldCodes(t *testing.T) {
	svc, _ := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	setup, err := svc.PrepareTwoFactor(context.Background(), "1", TwoFactorSetupInput{
		CurrentPassword: "password-123",
	})
	if err != nil {
		t.Fatalf("PrepareTwoFactor: %v", err)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	enabled, err := svc.EnableTwoFactor(context.Background(), "1", EnableTwoFactorInput{Code: code})
	if err != nil {
		t.Fatalf("EnableTwoFactor: %v", err)
	}
	oldRecoveryCode := enabled.RecoveryCodes[0]

	regenerateCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode regenerate: %v", err)
	}
	regenerated, err := svc.RegenerateRecoveryCodes(context.Background(), "1", RegenerateRecoveryCodesInput{
		CurrentPassword: "password-123",
		Code:            regenerateCode,
	})
	if err != nil {
		t.Fatalf("RegenerateRecoveryCodes: %v", err)
	}
	if len(regenerated.RecoveryCodes) != security.RecoveryCodeCount {
		t.Fatalf("expected %d recovery codes, got %d", security.RecoveryCodeCount, len(regenerated.RecoveryCodes))
	}

	_, err = svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123", TwoFactorCode: oldRecoveryCode,
	}, "127.0.0.1")
	if appErr, ok := err.(*apperror.AppError); !ok || appErr.Code != "AUTH_2FA_INVALID" {
		t.Fatalf("expected old recovery code to fail, got %v", err)
	}
}

func TestAuthServiceTrustedDeviceSkipsMFA(t *testing.T) {
	svc, repo := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	setup, err := svc.PrepareTwoFactor(context.Background(), "1", TwoFactorSetupInput{
		CurrentPassword: "password-123",
	})
	if err != nil {
		t.Fatalf("PrepareTwoFactor: %v", err)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := svc.EnableTwoFactor(context.Background(), "1", EnableTwoFactorInput{Code: code}); err != nil {
		t.Fatalf("EnableTwoFactor: %v", err)
	}
	loginCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode login: %v", err)
	}
	firstLogin, err := svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123", TwoFactorCode: loginCode,
		RememberDevice: true, TrustedDeviceName: "test browser",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Login with 2FA: %v", err)
	}
	if firstLogin.TrustedDeviceToken == "" || firstLogin.TrustedDevice == nil {
		t.Fatalf("expected trusted device token")
	}
	secondLogin, err := svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123", TrustedDeviceToken: firstLogin.TrustedDeviceToken,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Login with trusted device: %v", err)
	}
	if secondLogin.Token == "" {
		t.Fatalf("expected token")
	}
	disableCode, err := totp.GenerateCode(setup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode disable: %v", err)
	}
	if _, err := svc.DisableTwoFactor(context.Background(), "1", DisableTwoFactorInput{
		CurrentPassword: "password-123",
		Code:            disableCode,
	}); err != nil {
		t.Fatalf("DisableTwoFactor: %v", err)
	}
	if repo.users[0].TrustedDevices != "" {
		t.Fatalf("expected trusted devices cleared after disabling last MFA method")
	}
}

func TestAuthServiceOutOfBandOTPLoginConsumesCode(t *testing.T) {
	svc, repo := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	user := repo.users[0]
	user.Email = "admin@example.com"
	user.EmailOTPEnabled = true
	hash, err := security.HashPassword("123456")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ciphertext, err := svc.twoFactorCipher.EncryptJSON(pendingOutOfBandOTP{
		Channel: "email", CodeHash: hash, ExpiresAt: time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("EncryptJSON: %v", err)
	}
	user.OutOfBandOTPCiphertext = ciphertext
	if err := repo.Update(context.Background(), user); err != nil {
		t.Fatalf("Update: %v", err)
	}

	loginResult, err := svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123", TwoFactorCode: "123456",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("Login with email OTP: %v", err)
	}
	if loginResult.Token == "" {
		t.Fatalf("expected token")
	}
	if repo.users[0].OutOfBandOTPCiphertext != "" {
		t.Fatalf("expected OTP to be consumed")
	}

	_, err = svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123", TwoFactorCode: "123456",
	}, "127.0.0.1")
	if appErr, ok := err.(*apperror.AppError); !ok || appErr.Code != "AUTH_2FA_INVALID" {
		t.Fatalf("expected consumed OTP to fail, got %v", err)
	}
}

func TestAuthServiceMFAStartIsRateLimited(t *testing.T) {
	svc, repo := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	repo.users[0].Email = "admin@example.com"
	repo.users[0].EmailOTPEnabled = true

	for i := 0; i < 5; i++ {
		_ = svc.SendLoginOTP(context.Background(), LoginOTPInput{
			Username: "admin", Password: "wrong-password", Channel: "email",
		}, "127.0.0.1")
	}
	err = svc.SendLoginOTP(context.Background(), LoginOTPInput{
		Username: "admin", Password: "wrong-password", Channel: "email",
	}, "127.0.0.1")
	if appErr, ok := err.(*apperror.AppError); !ok || appErr.Code != "AUTH_RATE_LIMITED" {
		t.Fatalf("expected AUTH_RATE_LIMITED, got %v", err)
	}
}

func TestAuthServiceDisabledOTPChannelCannotConsumePendingCode(t *testing.T) {
	svc, repo := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	user := repo.users[0]
	user.Email = "admin@example.com"
	user.EmailOTPEnabled = false
	user.SMSOTPEnabled = true
	hash, err := security.HashPassword("123456")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ciphertext, err := svc.twoFactorCipher.EncryptJSON(pendingOutOfBandOTP{
		Channel: "email", CodeHash: hash, ExpiresAt: time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("EncryptJSON: %v", err)
	}
	user.OutOfBandOTPCiphertext = ciphertext
	if err := repo.Update(context.Background(), user); err != nil {
		t.Fatalf("Update: %v", err)
	}

	_, err = svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123", TwoFactorCode: "123456",
	}, "127.0.0.1")
	if appErr, ok := err.(*apperror.AppError); !ok || appErr.Code != "AUTH_2FA_INVALID" {
		t.Fatalf("expected disabled OTP channel to fail, got %v", err)
	}
	if repo.users[0].OutOfBandOTPCiphertext != "" {
		t.Fatalf("expected disabled channel OTP to be cleared")
	}
}

func TestAuthServiceChangingOTPRecipientClearsPendingCode(t *testing.T) {
	svc, repo := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	user := repo.users[0]
	user.Email = "old@example.com"
	user.EmailOTPEnabled = true
	hash, err := security.HashPassword("123456")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ciphertext, err := svc.twoFactorCipher.EncryptJSON(pendingOutOfBandOTP{
		Channel: "email", CodeHash: hash, ExpiresAt: time.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("EncryptJSON: %v", err)
	}
	user.OutOfBandOTPCiphertext = ciphertext
	if err := repo.Update(context.Background(), user); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := svc.ConfigureOutOfBandOTP(context.Background(), "1", OTPConfigInput{
		CurrentPassword: "password-123",
		Channel:         "email",
		Enabled:         true,
		Email:           "new@example.com",
	})
	if err != nil {
		t.Fatalf("ConfigureOutOfBandOTP: %v", err)
	}
	if updated.Email != "new@example.com" {
		t.Fatalf("expected email updated, got %q", updated.Email)
	}
	if repo.users[0].OutOfBandOTPCiphertext != "" {
		t.Fatalf("expected pending email OTP to be cleared after recipient change")
	}
}

func TestAuthServiceCorruptWebAuthnCredentialsStillRequireMFA(t *testing.T) {
	svc, repo := newTestAuthService()
	_, err := svc.Setup(context.Background(), SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	})
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	repo.users[0].WebAuthnCredentials = "{invalid-json"

	_, err = svc.Login(context.Background(), LoginInput{
		Username: "admin", Password: "password-123",
	}, "127.0.0.1")
	if appErr, ok := err.(*apperror.AppError); !ok || appErr.Code != "AUTH_2FA_REQUIRED" {
		t.Fatalf("expected corrupt WebAuthn credentials to require MFA, got %v", err)
	}
}
