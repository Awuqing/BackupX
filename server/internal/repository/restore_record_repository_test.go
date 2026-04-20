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

func newRestoreRecordTestRepository(t *testing.T) (*GormRestoreRecordRepository, uint) {
	t.Helper()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	storageTarget := &model.StorageTarget{Name: "local", Type: "local_disk", Enabled: true, ConfigCiphertext: "{}", ConfigVersion: 1, LastTestStatus: "unknown"}
	if err := db.Create(storageTarget).Error; err != nil {
		t.Fatalf("seed storage target error: %v", err)
	}
	task := &model.BackupTask{Name: "website", Type: "file", Enabled: true, SourcePath: "/srv/www", StorageTargetID: storageTarget.ID, RetentionDays: 30, Compression: "gzip", MaxBackups: 10, LastStatus: "idle"}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("seed backup task error: %v", err)
	}
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
	backupRecord := &model.BackupRecord{TaskID: task.ID, StorageTargetID: storageTarget.ID, Status: model.BackupRecordStatusSuccess, FileName: "website.tar.gz", FileSize: 1024, StoragePath: "tasks/1/website.tar.gz", StartedAt: now, CompletedAt: &completedAt}
	if err := db.Create(backupRecord).Error; err != nil {
		t.Fatalf("seed backup record error: %v", err)
	}
	return NewRestoreRecordRepository(db), backupRecord.ID
}

func TestRestoreRecordRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	repo, backupRecordID := newRestoreRecordTestRepository(t)

	startedAt := time.Now().UTC()
	restore := &model.RestoreRecord{
		BackupRecordID: backupRecordID,
		TaskID:         1,
		NodeID:         0,
		Status:         model.RestoreRecordStatusRunning,
		StartedAt:      startedAt,
		TriggeredBy:    "admin",
	}
	if err := repo.Create(ctx, restore); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if restore.ID == 0 {
		t.Fatalf("expected generated restore ID, got 0")
	}

	found, err := repo.FindByID(ctx, restore.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if found == nil || found.TriggeredBy != "admin" || found.Status != model.RestoreRecordStatusRunning {
		t.Fatalf("unexpected restore record: %#v", found)
	}
	if found.BackupRecord.ID != backupRecordID {
		t.Fatalf("expected BackupRecord preload, got %#v", found.BackupRecord)
	}

	completedAt := startedAt.Add(30 * time.Second)
	found.Status = model.RestoreRecordStatusSuccess
	found.DurationSeconds = 30
	found.CompletedAt = &completedAt
	if err := repo.Update(ctx, found); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	runningFilter := model.RestoreRecordStatusRunning
	list, err := repo.List(ctx, RestoreRecordListOptions{Status: runningFilter})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no running restores after update, got %d", len(list))
	}

	successFilter := model.RestoreRecordStatusSuccess
	successList, err := repo.List(ctx, RestoreRecordListOptions{Status: successFilter})
	if err != nil {
		t.Fatalf("List success returned error: %v", err)
	}
	if len(successList) != 1 {
		t.Fatalf("expected 1 success restore, got %d", len(successList))
	}

	brID := backupRecordID
	byBackup, err := repo.List(ctx, RestoreRecordListOptions{BackupRecordID: &brID})
	if err != nil {
		t.Fatalf("List byBackup returned error: %v", err)
	}
	if len(byBackup) != 1 {
		t.Fatalf("expected 1 restore for backup record, got %d", len(byBackup))
	}

	total, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 total, got %d", total)
	}

	if err := repo.Delete(ctx, restore.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	afterDel, err := repo.FindByID(ctx, restore.ID)
	if err != nil {
		t.Fatalf("FindByID after delete returned error: %v", err)
	}
	if afterDel != nil {
		t.Fatalf("expected nil after delete, got %#v", afterDel)
	}
}
