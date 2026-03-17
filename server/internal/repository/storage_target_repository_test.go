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

func openTestDB(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}

func newStorageTestRepository(t *testing.T) *GormStorageTargetRepository {
	t.Helper()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	return NewStorageTargetRepository(db)
}

func TestStorageTargetRepositoryCRUD(t *testing.T) {
	ctx := openTestDB(t)
	repo := newStorageTestRepository(t)
	item := &model.StorageTarget{
		Name:             "local",
		Type:             "local_disk",
		Enabled:          true,
		ConfigCiphertext: "ciphertext",
		ConfigVersion:    1,
		LastTestStatus:   "unknown",
	}
	if err := repo.Create(ctx, item); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	stored, err := repo.FindByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if stored == nil || stored.Name != "local" {
		t.Fatalf("unexpected stored target: %#v", stored)
	}
	byName, err := repo.FindByName(ctx, "local")
	if err != nil {
		t.Fatalf("FindByName returned error: %v", err)
	}
	if byName == nil || byName.ID != item.ID {
		t.Fatalf("expected target lookup by name to match, got %#v", byName)
	}
	stored.Description = "updated"
	if err := repo.Update(ctx, stored); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	items, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].Description != "updated" {
		t.Fatalf("unexpected list result: %#v", items)
	}
	if err := repo.Delete(ctx, item.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	deleted, err := repo.FindByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("FindByID after delete returned error: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected target to be deleted, got %#v", deleted)
	}
}
