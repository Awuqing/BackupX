package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"backupx/server/internal/backup"
	backupretention "backupx/server/internal/backup/retention"
	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
	"backupx/server/internal/storage/localdisk"
)

func newExecutionTestServices(t *testing.T) (*BackupExecutionService, *BackupRecordService, repository.BackupTaskRepository, repository.StorageTargetRepository, repository.BackupRecordRepository, string, string) {
	t.Helper()
	baseDir := t.TempDir()
	storageDir := filepath.Join(baseDir, "storage")
	sourceDir := filepath.Join(baseDir, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "index.html"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(baseDir, "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	cipher := codec.NewConfigCipher("execution-secret")
	tasks := repository.NewBackupTaskRepository(db)
	targets := repository.NewStorageTargetRepository(db)
	records := repository.NewBackupRecordRepository(db)
	configCiphertext, err := cipher.EncryptJSON(map[string]any{"basePath": storageDir})
	if err != nil {
		t.Fatalf("EncryptJSON returned error: %v", err)
	}
	if err := targets.Create(context.Background(), &model.StorageTarget{Name: "local", Type: string(storage.ProviderTypeLocalDisk), Enabled: true, ConfigCiphertext: configCiphertext, ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatalf("Create storage target returned error: %v", err)
	}
	if err := tasks.Create(context.Background(), &model.BackupTask{Name: "site-files", Type: "file", Enabled: true, SourcePath: sourceDir, StorageTargetID: 1, RetentionDays: 30, Compression: "gzip", MaxBackups: 10, LastStatus: "idle"}); err != nil {
		t.Fatalf("Create backup task returned error: %v", err)
	}
	logHub := backup.NewLogHub()
	runnerRegistry := backup.NewRegistry(backup.NewFileRunner(), backup.NewMySQLRunner(nil), backup.NewSQLiteRunner(), backup.NewPostgreSQLRunner(nil))
	storageRegistry := storage.NewRegistry(localdisk.NewFactory())
	retentionService := backupretention.NewService(records)
	executionService := NewBackupExecutionService(tasks, records, targets, storageRegistry, runnerRegistry, logHub, retentionService, cipher, nil, "", 2)
	recordService := NewBackupRecordService(records, executionService, logHub)
	return executionService, recordService, tasks, targets, records, sourceDir, storageDir
}

func TestBackupExecutionServiceRunTaskByIDSync(t *testing.T) {
	executionService, _, _, _, records, _, storageDir := newExecutionTestServices(t)
	detail, err := executionService.RunTaskByIDSync(context.Background(), 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync returned error: %v", err)
	}
	if detail.Status != "success" || detail.StoragePath == "" {
		t.Fatalf("unexpected record detail: %#v", detail)
	}
	stored, err := records.FindByID(context.Background(), detail.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if stored == nil || stored.Status != "success" {
		t.Fatalf("unexpected stored record: %#v", stored)
	}
	if _, err := os.Stat(filepath.Join(storageDir, filepath.FromSlash(detail.StoragePath))); err != nil {
		t.Fatalf("expected artifact in local storage: %v", err)
	}
}

func TestBackupRecordServiceRestore(t *testing.T) {
	executionService, recordService, _, _, _, sourceDir, _ := newExecutionTestServices(t)
	detail, err := executionService.RunTaskByIDSync(context.Background(), 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync returned error: %v", err)
	}
	if err := os.RemoveAll(sourceDir); err != nil {
		t.Fatalf("RemoveAll returned error: %v", err)
	}
	if err := recordService.Restore(context.Background(), detail.ID); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(sourceDir, "index.html"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected restored content: %s", string(content))
	}
}
