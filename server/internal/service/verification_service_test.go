package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

type verifyTestHarness struct {
	verify     *VerificationService
	execution  *BackupExecutionService
	records    repository.BackupRecordRepository
	storageDir string
}

func newVerifyTestHarness(t *testing.T) *verifyTestHarness {
	t.Helper()
	baseDir := t.TempDir()
	sourceDir := filepath.Join(baseDir, "source")
	storageDir := filepath.Join(baseDir, "storage")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "index.html"), []byte("hello-verify"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(baseDir, "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	cipher := codec.NewConfigCipher("verify-secret")
	targets := repository.NewStorageTargetRepository(db)
	tasks := repository.NewBackupTaskRepository(db)
	records := repository.NewBackupRecordRepository(db)
	verifications := repository.NewVerificationRecordRepository(db)
	nodes := repository.NewNodeRepository(db)
	targetCipher, err := cipher.EncryptJSON(map[string]any{"basePath": storageDir})
	if err != nil {
		t.Fatalf("EncryptJSON: %v", err)
	}
	if err := targets.Create(context.Background(), &model.StorageTarget{Name: "local", Type: string(storage.ProviderTypeLocalDisk), Enabled: true, ConfigCiphertext: targetCipher, ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatalf("create target: %v", err)
	}
	task := &model.BackupTask{Name: "verify-test", Type: "file", Enabled: true, SourcePath: sourceDir, StorageTargetID: 1, NodeID: 0, RetentionDays: 30, Compression: "gzip", MaxBackups: 10, LastStatus: "idle"}
	if err := tasks.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	logHub := backup.NewLogHub()
	runnerRegistry := backup.NewRegistry(backup.NewFileRunner(), backup.NewSQLiteRunner(), backup.NewMySQLRunner(nil), backup.NewPostgreSQLRunner(nil))
	storageRegistry := storage.NewRegistry(storageRclone.NewLocalDiskFactory())
	execution := NewBackupExecutionService(tasks, records, targets, storageRegistry, runnerRegistry, logHub, nil, cipher, nil, baseDir, 2, 10, "")
	verify := NewVerificationService(verifications, records, tasks, targets, nodes, storageRegistry, backup.NewLogHub(), cipher, baseDir, 2)

	return &verifyTestHarness{verify: verify, execution: execution, records: records, storageDir: storageDir}
}

// runVerify 同步执行一次验证并返回终态记录。
func (h *verifyTestHarness) runVerify(t *testing.T, backupRecordID uint) *VerificationRecordDetail {
	t.Helper()
	ctx := context.Background()
	done := make(chan struct{})
	h.verify.async = func(job func()) {
		go func() { job(); close(done) }()
	}
	detail, err := h.verify.Start(ctx, backupRecordID, "quick", "tester")
	if err != nil {
		t.Fatalf("verify Start: %v", err)
	}
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("verify did not complete in time")
	}
	final, err := h.verify.Get(ctx, detail.ID)
	if err != nil {
		t.Fatalf("verify Get: %v", err)
	}
	return final
}

// TestVerificationService_Success 覆盖正常路径：对一个有效（gzip 压缩）的备份做验证应通过。
// 同时回归保护 #77——新增的 SHA-256 校验不得误伤合法的压缩备份。
func TestVerificationService_Success(t *testing.T) {
	h := newVerifyTestHarness(t)
	backupDetail, err := h.execution.RunTaskByIDSync(context.Background(), 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync: %v", err)
	}
	if backupDetail.Status != "success" {
		t.Fatalf("expected backup success, got %s", backupDetail.Status)
	}

	final := h.runVerify(t, backupDetail.ID)
	if final.Status != model.VerificationRecordStatusSuccess {
		t.Fatalf("expected verify success, got %s (err=%s)", final.Status, final.ErrorMessage)
	}
}

// TestVerificationService_RejectsCorruptedBackup 验证 #77 的完整性校验：
// 存储对象被损坏时验证必须失败并给出 checksum 失败信息。
func TestVerificationService_RejectsCorruptedBackup(t *testing.T) {
	h := newVerifyTestHarness(t)
	backupDetail, err := h.execution.RunTaskByIDSync(context.Background(), 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync: %v", err)
	}
	if backupDetail.Status != "success" {
		t.Fatalf("expected backup success, got %s", backupDetail.Status)
	}

	// 破坏已存储的备份对象，使其 SHA-256 与记录不符。
	corrupted := false
	if walkErr := filepath.Walk(h.storageDir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		f, openErr := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
		if openErr != nil {
			return openErr
		}
		defer f.Close()
		if _, writeErr := f.WriteString("corrupt"); writeErr != nil {
			return writeErr
		}
		corrupted = true
		return nil
	}); walkErr != nil {
		t.Fatalf("corrupt walk: %v", walkErr)
	}
	if !corrupted {
		t.Fatal("did not find a stored backup object to corrupt")
	}

	final := h.runVerify(t, backupDetail.ID)
	if final.Status != model.VerificationRecordStatusFailed {
		t.Fatalf("expected verify to FAIL on corrupted backup, got %s", final.Status)
	}
	if !strings.Contains(final.ErrorMessage, "完整性校验失败") && !strings.Contains(final.ErrorMessage, "SHA-256") {
		t.Fatalf("expected checksum failure message, got %q", final.ErrorMessage)
	}
}
