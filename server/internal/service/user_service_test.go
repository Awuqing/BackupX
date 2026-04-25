package service

import (
	"context"
	"testing"

	"backupx/server/internal/model"
	"backupx/server/internal/security"
)

func TestUserServiceUpdatePasswordClearsTrustedDeviceState(t *testing.T) {
	hash, err := security.HashPassword("old-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	repo := &fakeUserRepository{users: []*model.User{{
		ID:                          1,
		Username:                    "admin",
		PasswordHash:                hash,
		DisplayName:                 "Admin",
		Email:                       "admin@example.com",
		Role:                        model.UserRoleAdmin,
		TwoFactorEnabled:            true,
		TrustedDevices:              `[{"id":"device"}]`,
		OutOfBandOTPCiphertext:      "pending",
		WebAuthnChallengeCiphertext: "challenge",
	}}}
	svc := NewUserService(repo)

	if _, err := svc.Update(context.Background(), 1, UserUpsertInput{
		Username:    "admin",
		Password:    "new-password",
		DisplayName: "Admin",
		Email:       "admin@example.com",
		Role:        model.UserRoleAdmin,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated := repo.users[0]
	if security.ComparePassword(updated.PasswordHash, "new-password") != nil {
		t.Fatalf("expected password hash to be updated")
	}
	if updated.TrustedDevices != "" || updated.OutOfBandOTPCiphertext != "" || updated.WebAuthnChallengeCiphertext != "" {
		t.Fatalf("expected password update to clear trusted device state, got trusted=%q otp=%q challenge=%q", updated.TrustedDevices, updated.OutOfBandOTPCiphertext, updated.WebAuthnChallengeCiphertext)
	}
}

func TestUserServiceUpdateContactClearsUnavailableOTP(t *testing.T) {
	hash, err := security.HashPassword("password-123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	repo := &fakeUserRepository{users: []*model.User{{
		ID:                     1,
		Username:               "admin",
		PasswordHash:           hash,
		DisplayName:            "Admin",
		Email:                  "admin@example.com",
		Phone:                  "+15550000000",
		Role:                   model.UserRoleAdmin,
		EmailOTPEnabled:        true,
		SMSOTPEnabled:          true,
		TrustedDevices:         `[{"id":"device"}]`,
		OutOfBandOTPCiphertext: "pending",
	}}}
	svc := NewUserService(repo)

	summary, err := svc.Update(context.Background(), 1, UserUpsertInput{
		Username:    "admin",
		DisplayName: "Admin",
		Role:        model.UserRoleAdmin,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated := repo.users[0]
	if updated.EmailOTPEnabled || updated.SMSOTPEnabled || summary.MFAEnabled {
		t.Fatalf("expected unavailable OTP channels to be disabled")
	}
	if updated.TrustedDevices != "" || updated.OutOfBandOTPCiphertext != "" || updated.WebAuthnChallengeCiphertext != "" {
		t.Fatalf("expected last MFA removal to clear temporary state")
	}
}

func TestUserServiceUpdateContactChangeClearsPendingOTP(t *testing.T) {
	hash, err := security.HashPassword("password-123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	repo := &fakeUserRepository{users: []*model.User{{
		ID:                     1,
		Username:               "admin",
		PasswordHash:           hash,
		DisplayName:            "Admin",
		Email:                  "old@example.com",
		Role:                   model.UserRoleAdmin,
		EmailOTPEnabled:        true,
		OutOfBandOTPCiphertext: "pending",
	}}}
	svc := NewUserService(repo)

	summary, err := svc.Update(context.Background(), 1, UserUpsertInput{
		Username:    "admin",
		DisplayName: "Admin",
		Email:       "new@example.com",
		Role:        model.UserRoleAdmin,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated := repo.users[0]
	if updated.Email != "new@example.com" || summary.Email != "new@example.com" {
		t.Fatalf("expected email to be updated")
	}
	if !updated.EmailOTPEnabled {
		t.Fatalf("expected email OTP to remain enabled")
	}
	if updated.OutOfBandOTPCiphertext != "" {
		t.Fatalf("expected contact change to clear pending OTP")
	}
}
