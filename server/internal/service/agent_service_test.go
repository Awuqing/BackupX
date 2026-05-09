package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage/codec"
)

func newAgentServicePoolTestHarness(t *testing.T) (*AgentService, repository.BackupRecordRepository, *model.Node, *model.Node) {
	t.Helper()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	cipher := codec.NewConfigCipher("agent-service-secret")
	nodeRepo := repository.NewNodeRepository(db)
	taskRepo := repository.NewBackupTaskRepository(db)
	recordRepo := repository.NewBackupRecordRepository(db)
	storageRepo := repository.NewStorageTargetRepository(db)
	cmdRepo := repository.NewAgentCommandRepository(db)

	owner := &model.Node{Name: "edge-owner", Token: "owner-token", Status: model.NodeStatusOnline, IsLocal: false, LastSeen: time.Now().UTC()}
	other := &model.Node{Name: "edge-other", Token: "other-token", Status: model.NodeStatusOnline, IsLocal: false, LastSeen: time.Now().UTC()}
	if err := nodeRepo.Create(context.Background(), owner); err != nil {
		t.Fatalf("create owner node: %v", err)
	}
	if err := nodeRepo.Create(context.Background(), other); err != nil {
		t.Fatalf("create other node: %v", err)
	}
	targetConfig, err := cipher.EncryptJSON(map[string]any{"basePath": t.TempDir()})
	if err != nil {
		t.Fatalf("EncryptJSON returned error: %v", err)
	}
	target := &model.StorageTarget{Name: "local", Type: "local_disk", Enabled: true, ConfigCiphertext: targetConfig, ConfigVersion: 1, LastTestStatus: "unknown"}
	if err := storageRepo.Create(context.Background(), target); err != nil {
		t.Fatalf("create storage target: %v", err)
	}
	task := &model.BackupTask{
		Name:            "pooled-task",
		Type:            "file",
		Enabled:         true,
		SourcePath:      "/srv/data",
		StorageTargetID: target.ID,
		NodeID:          0,
		NodePoolTag:     "db",
		RetentionDays:   30,
		Compression:     "gzip",
		MaxBackups:      10,
		LastStatus:      "running",
	}
	if err := taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	record := &model.BackupRecord{
		TaskID:          task.ID,
		StorageTargetID: target.ID,
		NodeID:          owner.ID,
		Status:          model.BackupRecordStatusRunning,
		StartedAt:       time.Now().UTC(),
	}
	if err := recordRepo.Create(context.Background(), record); err != nil {
		t.Fatalf("create record: %v", err)
	}
	return NewAgentService(nodeRepo, taskRepo, recordRepo, storageRepo, cmdRepo, cipher), recordRepo, owner, other
}

func TestAgentServicePooledTaskUsesRecordNodeForSpecAndRecordUpdates(t *testing.T) {
	svc, records, owner, other := newAgentServicePoolTestHarness(t)
	ctx := context.Background()

	spec, err := svc.GetTaskSpec(ctx, owner, 1)
	if err != nil {
		t.Fatalf("owner GetTaskSpec returned error: %v", err)
	}
	if spec.TaskID != 1 || len(spec.StorageTargets) != 1 {
		t.Fatalf("unexpected spec: %#v", spec)
	}
	if _, err := svc.GetTaskSpec(ctx, other, 1); err == nil {
		t.Fatal("expected non-owner node to be forbidden from pooled task spec")
	}

	if err := svc.UpdateRecord(ctx, owner, 1, AgentRecordUpdate{
		Status:      model.BackupRecordStatusSuccess,
		FileName:    "backup.tar.gz",
		FileSize:    123,
		StoragePath: "tasks/1/backup.tar.gz",
	}); err != nil {
		t.Fatalf("owner UpdateRecord returned error: %v", err)
	}
	updated, err := records.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if updated.Status != model.BackupRecordStatusSuccess || updated.NodeID != owner.ID {
		t.Fatalf("unexpected updated record: %#v", updated)
	}
	if err := svc.UpdateRecord(ctx, other, 1, AgentRecordUpdate{LogAppend: "bad"}); err == nil {
		t.Fatal("expected non-owner node to be forbidden from record update")
	}
}
