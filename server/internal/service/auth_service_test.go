package service

import (
	"context"
	"testing"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/security"
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
