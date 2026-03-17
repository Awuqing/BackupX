package service

import (
	"context"
	"path/filepath"
	"testing"

	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage/codec"
)

func newBackupTaskServiceForTest(t *testing.T) (*BackupTaskService, repository.StorageTargetRepository, repository.BackupTaskRepository) {
	t.Helper()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	targets := repository.NewStorageTargetRepository(db)
	tasks := repository.NewBackupTaskRepository(db)
	service := NewBackupTaskService(tasks, targets, codec.NewConfigCipher("task-service-secret"))
	return service, targets, tasks
}

func TestBackupTaskServiceCreateAndGet(t *testing.T) {
	ctx := context.Background()
	service, targets, _ := newBackupTaskServiceForTest(t)
	if err := targets.Create(ctx, &model.StorageTarget{Name: "local", Type: "local_disk", Enabled: true, ConfigCiphertext: "ciphertext", ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatalf("seed storage target error: %v", err)
	}
	created, err := service.Create(ctx, BackupTaskUpsertInput{
		Name:            "site-files",
		Type:            "file",
		Enabled:         true,
		SourcePath:      "/srv/site",
		ExcludePatterns: []string{"*.log", "node_modules"},
		StorageTargetID: 1,
		RetentionDays:   30,
		Compression:     "gzip",
		MaxBackups:      10,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.Name != "site-files" || len(created.ExcludePatterns) != 2 {
		t.Fatalf("unexpected created task: %#v", created)
	}
	loaded, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if loaded.StorageTargetName != "local" {
		t.Fatalf("expected storage target name local, got %s", loaded.StorageTargetName)
	}
}

func TestBackupTaskServiceKeepsMaskedPasswordOnUpdate(t *testing.T) {
	ctx := context.Background()
	service, targets, tasks := newBackupTaskServiceForTest(t)
	if err := targets.Create(ctx, &model.StorageTarget{Name: "local", Type: "local_disk", Enabled: true, ConfigCiphertext: "ciphertext", ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatalf("seed storage target error: %v", err)
	}
	created, err := service.Create(ctx, BackupTaskUpsertInput{
		Name:            "mysql-prod",
		Type:            "mysql",
		Enabled:         true,
		DBHost:          "127.0.0.1",
		DBPort:          3306,
		DBUser:          "root",
		DBPassword:      "secret",
		DBName:          "app",
		StorageTargetID: 1,
		RetentionDays:   7,
		Compression:     "gzip",
		MaxBackups:      5,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	stored, err := tasks.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	originalCiphertext := stored.DBPasswordCiphertext
	updated, err := service.Update(ctx, created.ID, BackupTaskUpsertInput{
		Name:            created.Name,
		Type:            created.Type,
		Enabled:         true,
		DBHost:          "127.0.0.1",
		DBPort:          3306,
		DBUser:          "root",
		DBPassword:      "",
		DBName:          "app_updated",
		StorageTargetID: 1,
		RetentionDays:   7,
		Compression:     "gzip",
		MaxBackups:      5,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if len(updated.MaskedFields) != 1 || updated.MaskedFields[0] != "dbPassword" {
		t.Fatalf("expected masked dbPassword field, got %#v", updated.MaskedFields)
	}
	reloaded, err := tasks.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if reloaded.DBPasswordCiphertext != originalCiphertext {
		t.Fatalf("expected ciphertext unchanged")
	}
}
