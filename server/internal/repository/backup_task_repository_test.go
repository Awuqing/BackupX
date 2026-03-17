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

func newBackupTaskTestRepository(t *testing.T) *GormBackupTaskRepository {
	t.Helper()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	if err := db.Create(&model.StorageTarget{Name: "local", Type: "local_disk", Enabled: true, ConfigCiphertext: "{}", ConfigVersion: 1, LastTestStatus: "unknown"}).Error; err != nil {
		t.Fatalf("seed storage target error: %v", err)
	}
	return NewBackupTaskRepository(db)
}

func TestBackupTaskRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	repo := newBackupTaskTestRepository(t)
	task := &model.BackupTask{
		Name:            "website",
		Type:            "file",
		Enabled:         true,
		SourcePath:      "/srv/www/site",
		StorageTargetID: 1,
		RetentionDays:   30,
		Compression:     "gzip",
		MaxBackups:      10,
		LastStatus:      "idle",
	}
	if err := repo.Create(ctx, task); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	stored, err := repo.FindByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if stored == nil || stored.Name != "website" {
		t.Fatalf("unexpected stored task: %#v", stored)
	}
	stored.Enabled = false
	stored.CronExpr = "0 3 * * *"
	if err := repo.Update(ctx, stored); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	schedulable, err := repo.ListSchedulable(ctx)
	if err != nil {
		t.Fatalf("ListSchedulable returned error: %v", err)
	}
	if len(schedulable) != 0 {
		t.Fatalf("expected disabled task not schedulable, got %d", len(schedulable))
	}
	stored.Enabled = true
	if err := repo.Update(ctx, stored); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	schedulable, err = repo.ListSchedulable(ctx)
	if err != nil {
		t.Fatalf("ListSchedulable returned error: %v", err)
	}
	if len(schedulable) != 1 {
		t.Fatalf("expected one schedulable task, got %d", len(schedulable))
	}
	count, err := repo.CountByStorageTargetID(ctx, 1)
	if err != nil {
		t.Fatalf("CountByStorageTargetID returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected referenced task count 1, got %d", count)
	}
	if err := repo.Delete(ctx, task.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	deleted, err := repo.FindByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("FindByID after delete returned error: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected task deleted, got %#v", deleted)
	}
}
