package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backupx/server/internal/backup"
	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
	storageRclone "backupx/server/internal/storage/rclone"
)

func newLockTestHarness(t *testing.T) (*BackupRecordService, *BackupExecutionService) {
	t.Helper()
	baseDir := t.TempDir()
	sourceDir := filepath.Join(baseDir, "data")
	storeDir := filepath.Join(baseDir, "store")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "f.txt"), []byte("lock-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(baseDir, "backupx.db")}, log)
	if err != nil {
		t.Fatal(err)
	}
	cipher := codec.NewConfigCipher("lock-secret")
	targets := repository.NewStorageTargetRepository(db)
	tasks := repository.NewBackupTaskRepository(db)
	records := repository.NewBackupRecordRepository(db)
	cfg, err := cipher.EncryptJSON(map[string]any{"basePath": storeDir})
	if err != nil {
		t.Fatal(err)
	}
	if err := targets.Create(context.Background(), &model.StorageTarget{Name: "s", Type: string(storage.ProviderTypeLocalDisk), Enabled: true, ConfigCiphertext: cfg, ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatal(err)
	}
	task := &model.BackupTask{Name: "lock-task", Type: "file", Enabled: true, SourcePath: sourceDir, StorageTargetID: 1, NodeID: 0, RetentionDays: 30, Compression: "gzip", MaxBackups: 10, LastStatus: "idle"}
	if err := tasks.Create(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	logHub := backup.NewLogHub()
	runnerRegistry := backup.NewRegistry(backup.NewFileRunner(), backup.NewSQLiteRunner(), backup.NewMySQLRunner(nil), backup.NewPostgreSQLRunner(nil))
	storageRegistry := storage.NewRegistry(storageRclone.NewLocalDiskFactory())
	execution := NewBackupExecutionService(tasks, records, targets, storageRegistry, runnerRegistry, logHub, nil, cipher, nil, baseDir, 2, 10, "")
	recordService := NewBackupRecordService(records, execution, logHub)
	return recordService, execution
}

// TestBackupRecordLock_BlocksDeletion 验证保留锁定后手动删除被拒绝，解锁后可删除。
func TestBackupRecordLock_BlocksDeletion(t *testing.T) {
	recordService, execution := newLockTestHarness(t)
	ctx := context.Background()

	bd, err := execution.RunTaskByIDSync(ctx, 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync: %v", err)
	}
	if bd.Status != "success" {
		t.Fatalf("backup not success: %s", bd.Status)
	}

	// 锁定。
	detail, err := recordService.SetLock(ctx, bd.ID, true)
	if err != nil {
		t.Fatalf("SetLock(true): %v", err)
	}
	if !detail.Locked {
		t.Fatal("expected detail.Locked = true")
	}

	// 锁定状态下删除应被拒绝。
	if err := execution.DeleteRecord(ctx, bd.ID); err == nil {
		t.Fatal("expected delete of locked record to be rejected")
	} else if !strings.Contains(err.Error(), "保留锁定") {
		t.Fatalf("unexpected delete error: %v", err)
	}

	// 记录仍然存在。
	if got, _ := recordService.Get(ctx, bd.ID); got == nil {
		t.Fatal("locked record must still exist after rejected delete")
	}

	// 解锁后可删除。
	if _, err := recordService.SetLock(ctx, bd.ID, false); err != nil {
		t.Fatalf("SetLock(false): %v", err)
	}
	if err := execution.DeleteRecord(ctx, bd.ID); err != nil {
		t.Fatalf("delete after unlock should succeed: %v", err)
	}
}
