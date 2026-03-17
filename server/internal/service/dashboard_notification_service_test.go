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
	"backupx/server/internal/notify"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage/codec"
)

type fakeNotifier struct {
	typeName   string
	messages   []notify.Message
	lastConfig map[string]any
}

func (n *fakeNotifier) Type() string              { return n.typeName }
func (n *fakeNotifier) SensitiveFields() []string { return []string{"secret"} }
func (n *fakeNotifier) Validate(config map[string]any) error {
	if config["url"] == nil {
		return nil
	}
	return nil
}
func (n *fakeNotifier) Send(_ context.Context, config map[string]any, message notify.Message) error {
	n.lastConfig = config
	n.messages = append(n.messages, message)
	return nil
}

func newDashboardNotificationTestDeps(t *testing.T) (*DashboardService, *NotificationService, *fakeNotifier, repository.BackupTaskRepository, repository.BackupRecordRepository, repository.NotificationRepository) {
	t.Helper()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	tasks := repository.NewBackupTaskRepository(db)
	records := repository.NewBackupRecordRepository(db)
	targets := repository.NewStorageTargetRepository(db)
	notifications := repository.NewNotificationRepository(db)
	if err := targets.Create(context.Background(), &model.StorageTarget{Name: "local", Type: "local_disk", Enabled: true, ConfigCiphertext: "ciphertext", ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatalf("Create storage target returned error: %v", err)
	}
	fake := &fakeNotifier{typeName: "webhook"}
	registry := notify.NewRegistry(fake)
	cipher := codec.NewConfigCipher("notify-secret")
	dashboardService := NewDashboardService(tasks, records, targets)
	notificationService := NewNotificationService(notifications, registry, cipher)
	return dashboardService, notificationService, fake, tasks, records, notifications
}

func TestDashboardServiceStats(t *testing.T) {
	dashboardService, _, _, tasks, records, _ := newDashboardNotificationTestDeps(t)
	ctx := context.Background()
	if err := tasks.Create(ctx, &model.BackupTask{Name: "site", Type: "file", Enabled: true, SourcePath: "/srv/site", StorageTargetID: 1, RetentionDays: 30, Compression: "gzip", MaxBackups: 10, LastStatus: "success"}); err != nil {
		t.Fatalf("Create task returned error: %v", err)
	}
	startedAt := time.Now().UTC().Add(-time.Hour)
	completedAt := time.Now().UTC()
	if err := records.Create(ctx, &model.BackupRecord{TaskID: 1, StorageTargetID: 1, Status: "success", FileName: "site.tar.gz", FileSize: 2048, StoragePath: "site/2026/03/07/site.tar.gz", DurationSeconds: 30, StartedAt: startedAt, CompletedAt: &completedAt}); err != nil {
		t.Fatalf("Create record returned error: %v", err)
	}
	stats, err := dashboardService.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats returned error: %v", err)
	}
	if stats.TotalTasks != 1 || stats.TotalRecords != 1 || stats.TotalBackupBytes != 2048 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if len(stats.RecentRecords) != 1 || len(stats.StorageUsage) != 1 {
		t.Fatalf("expected recent records and storage usage, got %#v", stats)
	}
}

func TestNotificationServiceCreateAndDispatch(t *testing.T) {
	_, notificationService, fake, _, _, notifications := newDashboardNotificationTestDeps(t)
	ctx := context.Background()
	created, err := notificationService.Create(ctx, NotificationUpsertInput{Name: "ops", Type: "webhook", Enabled: true, OnSuccess: true, OnFailure: true, Config: map[string]any{"url": "https://example.invalid", "secret": "top-secret"}})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if len(created.MaskedFields) != 1 || created.MaskedFields[0] != "secret" {
		t.Fatalf("unexpected masked fields: %#v", created.MaskedFields)
	}
	item, err := notifications.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if item == nil || item.ConfigCiphertext == "" {
		t.Fatalf("expected encrypted notification config")
	}
	if err := notificationService.NotifyBackupResult(ctx, BackupExecutionNotification{Task: &model.BackupTask{Name: "site"}, Record: &model.BackupRecord{ID: 1, Status: "success", StartedAt: time.Now().UTC()}, Error: nil}); err != nil {
		t.Fatalf("NotifyBackupResult returned error: %v", err)
	}
	if len(fake.messages) != 1 {
		t.Fatalf("expected one notification message, got %d", len(fake.messages))
	}
}
