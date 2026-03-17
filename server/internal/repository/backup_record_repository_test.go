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

func newBackupRecordTestRepository(t *testing.T) *GormBackupRecordRepository {
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
	task := &model.BackupTask{Name: "website", Type: "file", Enabled: true, SourcePath: "/srv/www/site", StorageTargetID: storageTarget.ID, RetentionDays: 30, Compression: "gzip", MaxBackups: 10, LastStatus: "idle"}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("seed backup task error: %v", err)
	}
	return NewBackupRecordRepository(db)
}

func TestBackupRecordRepositoryQueries(t *testing.T) {
	ctx := context.Background()
	repo := newBackupRecordTestRepository(t)
	now := time.Now().UTC()
	completedAt := now.Add(2 * time.Minute)
	record := &model.BackupRecord{
		TaskID:          1,
		StorageTargetID: 1,
		Status:          "success",
		FileName:        "website.tar.gz",
		FileSize:        1024,
		StoragePath:     "tasks/1/website.tar.gz",
		DurationSeconds: 120,
		LogContent:      "done",
		StartedAt:       now,
		CompletedAt:     &completedAt,
	}
	if err := repo.Create(ctx, record); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	stored, err := repo.FindByID(ctx, record.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if stored == nil || stored.FileName != "website.tar.gz" {
		t.Fatalf("unexpected stored record: %#v", stored)
	}
	listed, err := repo.List(ctx, BackupRecordListOptions{TaskID: &record.TaskID, Status: "success"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one listed record, got %d", len(listed))
	}
	recent, err := repo.ListRecent(ctx, 5)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected one recent record, got %d", len(recent))
	}
	total, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total count 1, got %d", total)
	}
	successCount, err := repo.CountSuccessSince(ctx, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("CountSuccessSince returned error: %v", err)
	}
	if successCount != 1 {
		t.Fatalf("expected success count 1, got %d", successCount)
	}
	sum, err := repo.SumFileSize(ctx)
	if err != nil {
		t.Fatalf("SumFileSize returned error: %v", err)
	}
	if sum != 1024 {
		t.Fatalf("expected file size sum 1024, got %d", sum)
	}
	timeline, err := repo.TimelineSince(ctx, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("TimelineSince returned error: %v", err)
	}
	if len(timeline) != 1 || timeline[0].Success != 1 {
		t.Fatalf("unexpected timeline: %#v", timeline)
	}
	usage, err := repo.StorageUsage(ctx)
	if err != nil {
		t.Fatalf("StorageUsage returned error: %v", err)
	}
	if len(usage) != 1 || usage[0].TotalSize != 1024 {
		t.Fatalf("unexpected usage: %#v", usage)
	}
	if err := repo.Delete(ctx, record.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}
