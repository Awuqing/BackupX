package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
)

func newOAuthSessionTestRepository(t *testing.T) *GormOAuthSessionRepository {
	t.Helper()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	return NewOAuthSessionRepository(db)
}

func TestOAuthSessionRepositoryCRUDAndDeleteExpired(t *testing.T) {
	ctx := context.Background()
	repo := newOAuthSessionTestRepository(t)
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	session := &model.OAuthSession{
		ProviderType:      "google_drive",
		State:             "oauth-state",
		PayloadCiphertext: "ciphertext",
		ExpiresAt:         expiresAt,
	}
	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	stored, err := repo.FindByState(ctx, "oauth-state")
	if err != nil {
		t.Fatalf("FindByState returned error: %v", err)
	}
	if stored == nil || stored.State != "oauth-state" {
		t.Fatalf("unexpected stored session: %#v", stored)
	}
	now := time.Now().UTC()
	stored.UsedAt = &now
	if err := repo.Update(ctx, stored); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if err := repo.DeleteExpired(ctx, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("DeleteExpired returned error: %v", err)
	}
	stillThere, err := repo.FindByState(ctx, "oauth-state")
	if err != nil {
		t.Fatalf("FindByState after DeleteExpired returned error: %v", err)
	}
	if stillThere == nil {
		t.Fatalf("expected unexpired session to remain")
	}
	if err := repo.DeleteExpired(ctx, time.Now().UTC().Add(10*time.Minute)); err != nil {
		t.Fatalf("DeleteExpired returned error: %v", err)
	}
	deleted, err := repo.FindByState(ctx, "oauth-state")
	if err != nil {
		t.Fatalf("FindByState after expiration delete returned error: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected session to be deleted, got %#v", deleted)
	}
}
