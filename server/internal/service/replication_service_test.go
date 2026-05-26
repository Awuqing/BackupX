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

type replicationTestHarness struct {
	repl      *ReplicationService
	execution *BackupExecutionService
	records   repository.BackupRecordRepository
	destDir   string
	srcDir    string
}

func newReplicationTestHarness(t *testing.T) *replicationTestHarness {
	t.Helper()
	baseDir := t.TempDir()
	sourceData := filepath.Join(baseDir, "data")
	srcStore := filepath.Join(baseDir, "src-store")
	destStore := filepath.Join(baseDir, "dest-store")
	if err := os.MkdirAll(sourceData, 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceData, "index.html"), []byte("hello-replicate"), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(baseDir, "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	cipher := codec.NewConfigCipher("replicate-secret")
	targets := repository.NewStorageTargetRepository(db)
	tasks := repository.NewBackupTaskRepository(db)
	records := repository.NewBackupRecordRepository(db)
	replications := repository.NewReplicationRecordRepository(db)
	nodes := repository.NewNodeRepository(db)

	mkTarget := func(name, basePath string) {
		cfg, err := cipher.EncryptJSON(map[string]any{"basePath": basePath})
		if err != nil {
			t.Fatalf("EncryptJSON: %v", err)
		}
		if err := targets.Create(context.Background(), &model.StorageTarget{Name: name, Type: string(storage.ProviderTypeLocalDisk), Enabled: true, ConfigCiphertext: cfg, ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
			t.Fatalf("create target %s: %v", name, err)
		}
	}
	mkTarget("src", srcStore)   // ID 1
	mkTarget("dest", destStore) // ID 2

	task := &model.BackupTask{Name: "repl-test", Type: "file", Enabled: true, SourcePath: sourceData, StorageTargetID: 1, NodeID: 0, RetentionDays: 30, Compression: "gzip", MaxBackups: 10, LastStatus: "idle"}
	if err := tasks.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	logHub := backup.NewLogHub()
	runnerRegistry := backup.NewRegistry(backup.NewFileRunner(), backup.NewSQLiteRunner(), backup.NewMySQLRunner(nil), backup.NewPostgreSQLRunner(nil))
	storageRegistry := storage.NewRegistry(storageRclone.NewLocalDiskFactory())
	execution := NewBackupExecutionService(tasks, records, targets, storageRegistry, runnerRegistry, logHub, nil, cipher, nil, baseDir, 2, 10, "")
	repl := NewReplicationService(replications, records, targets, nodes, storageRegistry, cipher, baseDir, 2)

	return &replicationTestHarness{repl: repl, execution: execution, records: records, destDir: destStore, srcDir: srcStore}
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// TestReplicationService_MirrorsToDestTarget 覆盖正常路径：把成功备份从源存储复制到目标存储，
// 目标出现对象、源保留（复制非移动），记录终态为 success。
func TestReplicationService_MirrorsToDestTarget(t *testing.T) {
	h := newReplicationTestHarness(t)
	ctx := context.Background()
	backupDetail, err := h.execution.RunTaskByIDSync(ctx, 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync: %v", err)
	}
	if backupDetail.Status != "success" {
		t.Fatalf("expected backup success, got %s", backupDetail.Status)
	}
	if countFiles(t, h.destDir) != 0 {
		t.Fatalf("dest store should be empty before replication")
	}

	done := make(chan struct{})
	h.repl.async = func(job func()) {
		go func() { job(); close(done) }()
	}
	summary, err := h.repl.Start(ctx, backupDetail.ID, 2, "tester")
	if err != nil {
		t.Fatalf("replication Start: %v", err)
	}
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("replication did not complete in time")
	}

	final, err := h.repl.Get(ctx, summary.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if final.Status != model.ReplicationStatusSuccess {
		t.Fatalf("expected replication success, got %s (err=%s)", final.Status, final.ErrorMessage)
	}
	if countFiles(t, h.destDir) == 0 {
		t.Fatal("dest store should contain the replicated object")
	}
	if countFiles(t, h.srcDir) == 0 {
		t.Fatal("source object must remain after replication (copy, not move)")
	}
}

// TestReplicationService_RejectsSameTarget 校验：目标与源相同时同步拒绝。
func TestReplicationService_RejectsSameTarget(t *testing.T) {
	h := newReplicationTestHarness(t)
	ctx := context.Background()
	backupDetail, err := h.execution.RunTaskByIDSync(ctx, 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync: %v", err)
	}
	// 备份写到 target 1；以 target 1 作为复制目标应被拒绝。
	if _, err := h.repl.Start(ctx, backupDetail.ID, 1, "tester"); err == nil {
		t.Fatal("expected error when dest target equals source")
	} else if !strings.Contains(err.Error(), "目标存储无效或与源相同") {
		t.Fatalf("unexpected error: %v", err)
	}
}
