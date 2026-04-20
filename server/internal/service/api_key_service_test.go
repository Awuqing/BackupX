package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

func newApiKeyTestService(t *testing.T) *ApiKeyService {
	t.Helper()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	return NewApiKeyService(repository.NewApiKeyRepository(db))
}

func TestApiKeyService_CreateAndAuthenticate(t *testing.T) {
	svc := newApiKeyTestService(t)
	ctx := context.Background()

	result, err := svc.Create(ctx, "tester", ApiKeyCreateInput{
		Name:     "ci",
		Role:     model.UserRoleOperator,
		TTLHours: 0,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasPrefix(result.PlainKey, ApiKeyPrefix) {
		t.Fatalf("expected plain key with prefix %s, got %s", ApiKeyPrefix, result.PlainKey)
	}
	if result.ApiKey.Role != model.UserRoleOperator {
		t.Fatalf("role not preserved")
	}

	subject, role, err := svc.Authenticate(ctx, result.PlainKey)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if role != model.UserRoleOperator {
		t.Fatalf("expected operator role, got %s", role)
	}
	if !strings.HasPrefix(subject, "api_key:") {
		t.Fatalf("expected subject to start with api_key:, got %s", subject)
	}
}

func TestApiKeyService_AuthenticateRejectsInvalid(t *testing.T) {
	svc := newApiKeyTestService(t)
	ctx := context.Background()

	// 格式错误（无 bax_ 前缀）
	if _, _, err := svc.Authenticate(ctx, "invalid-without-prefix"); err == nil {
		t.Fatalf("expected error for missing prefix")
	}
	// 格式正确但不存在
	if _, _, err := svc.Authenticate(ctx, "bax_"+strings.Repeat("0", 48)); err == nil {
		t.Fatalf("expected error for unknown key")
	}
}

func TestApiKeyService_AuthenticateRejectsExpired(t *testing.T) {
	svc := newApiKeyTestService(t)
	ctx := context.Background()

	result, err := svc.Create(ctx, "tester", ApiKeyCreateInput{
		Name:     "ci-expired",
		Role:     model.UserRoleViewer,
		TTLHours: 1,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// 手动把 expiresAt 设到过去
	key, _ := svc.repo.FindByID(ctx, result.ApiKey.ID)
	past := time.Now().UTC().Add(-time.Hour)
	key.ExpiresAt = &past
	if err := svc.repo.Update(ctx, key); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, _, err := svc.Authenticate(ctx, result.PlainKey); err == nil {
		t.Fatalf("expected error for expired key")
	}
}

func TestApiKeyService_AuthenticateRejectsDisabled(t *testing.T) {
	svc := newApiKeyTestService(t)
	ctx := context.Background()

	result, err := svc.Create(ctx, "tester", ApiKeyCreateInput{Name: "disabled", Role: "admin"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.ToggleDisabled(ctx, result.ApiKey.ID, true); err != nil {
		t.Fatalf("ToggleDisabled: %v", err)
	}
	if _, _, err := svc.Authenticate(ctx, result.PlainKey); err == nil {
		t.Fatalf("expected error for disabled key")
	}
}
