package repository

import (
	"context"
	"path/filepath"
	"testing"

	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
)

func newNotificationTestRepository(t *testing.T) *GormNotificationRepository {
	t.Helper()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	return NewNotificationRepository(db)
}

func TestNotificationRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationTestRepository(t)
	item := &model.Notification{
		Type:             "webhook",
		Name:             "ops-webhook",
		ConfigCiphertext: "ciphertext",
		Enabled:          true,
		OnSuccess:        false,
		OnFailure:        true,
	}
	if err := repo.Create(ctx, item); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	stored, err := repo.FindByName(ctx, "ops-webhook")
	if err != nil {
		t.Fatalf("FindByName returned error: %v", err)
	}
	if stored == nil || stored.Name != "ops-webhook" {
		t.Fatalf("unexpected notification: %#v", stored)
	}
	enabledForFailure, err := repo.ListEnabledForEvent(ctx, false)
	if err != nil {
		t.Fatalf("ListEnabledForEvent returned error: %v", err)
	}
	if len(enabledForFailure) != 1 {
		t.Fatalf("expected one failure notification, got %d", len(enabledForFailure))
	}
	stored.OnSuccess = true
	if err := repo.Update(ctx, stored); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	enabledForSuccess, err := repo.ListEnabledForEvent(ctx, true)
	if err != nil {
		t.Fatalf("ListEnabledForEvent returned error: %v", err)
	}
	if len(enabledForSuccess) != 1 {
		t.Fatalf("expected one success notification, got %d", len(enabledForSuccess))
	}
	if err := repo.Delete(ctx, item.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}
