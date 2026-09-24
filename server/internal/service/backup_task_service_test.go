package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"backupx/server/internal/apperror"
	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage/codec"
)

type backupTaskSchedulerStub struct {
	removeErr   error
	syncErr     error
	removedIDs  []uint
	syncedTasks []model.BackupTask
}

func (s *backupTaskSchedulerStub) SyncTask(_ context.Context, task *model.BackupTask) error {
	if task != nil {
		s.syncedTasks = append(s.syncedTasks, *task)
	}
	return s.syncErr
}

func (s *backupTaskSchedulerStub) RemoveTask(_ context.Context, taskID uint) error {
	s.removedIDs = append(s.removedIDs, taskID)
	return s.removeErr
}

type failingDeleteBackupTaskRepository struct {
	repository.BackupTaskRepository
	deleteErr error
}

func (r *failingDeleteBackupTaskRepository) Delete(context.Context, uint) error {
	return r.deleteErr
}

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
	closeTestDatabase(t, db)
	targets := repository.NewStorageTargetRepository(db)
	tasks := repository.NewBackupTaskRepository(db)
	service := NewBackupTaskService(tasks, targets, codec.NewConfigCipher("task-service-secret"))
	return service, targets, tasks
}

func TestBackupTaskServiceRejectsEncryptedRemoteTasks(t *testing.T) {
	ctx := context.Background()
	service, targets, _ := newBackupTaskServiceForTest(t)
	service.SetNodeRepository(&nodeRepoStub{nodes: []model.Node{
		{ID: 41, Name: "master", Token: "master-token", Status: model.NodeStatusOnline, IsLocal: true},
		{ID: 42, Name: "edge", Token: "edge-token", Status: model.NodeStatusOnline, IsLocal: false},
	}})
	if err := targets.Create(ctx, &model.StorageTarget{Name: "local", Type: "local_disk", Enabled: true, ConfigCiphertext: "ciphertext", ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatalf("seed storage target error: %v", err)
	}

	_, err := service.Create(ctx, BackupTaskUpsertInput{
		Name:            "encrypted-node-pool",
		Type:            "file",
		Enabled:         true,
		SourcePath:      "/srv/site",
		StorageTargetID: 1,
		NodePoolTag:     "db",
		RetentionDays:   30,
		Compression:     "gzip",
		MaxBackups:      10,
		Encrypt:         true,
	})
	if err == nil || !strings.Contains(err.Error(), "远程节点暂不支持加密备份") {
		t.Fatalf("expected encrypted node-pool task to be rejected, got %v", err)
	}

	created, err := service.Create(ctx, BackupTaskUpsertInput{
		Name:            "local-encrypted",
		Type:            "file",
		Enabled:         true,
		SourcePath:      "/srv/site",
		StorageTargetID: 1,
		RetentionDays:   30,
		Compression:     "gzip",
		MaxBackups:      10,
		Encrypt:         true,
	})
	if err != nil {
		t.Fatalf("Create local encrypted task returned error: %v", err)
	}
	localNodeTask, err := service.Create(ctx, BackupTaskUpsertInput{
		Name:            "local-node-encrypted",
		Type:            "file",
		Enabled:         true,
		SourcePath:      "/srv/site",
		StorageTargetID: 1,
		NodeID:          41,
		RetentionDays:   30,
		Compression:     "gzip",
		MaxBackups:      10,
		Encrypt:         true,
	})
	if err != nil {
		t.Fatalf("Create encrypted task pinned to local node returned error: %v", err)
	}
	if localNodeTask.NodeID != 41 || !localNodeTask.Encrypt {
		t.Fatalf("expected encrypted task to keep local node, got %#v", localNodeTask)
	}
	_, err = service.Update(ctx, created.ID, BackupTaskUpsertInput{
		Name:            created.Name,
		Type:            created.Type,
		Enabled:         true,
		SourcePath:      "/srv/site",
		StorageTargetID: 1,
		NodeID:          42,
		RetentionDays:   30,
		Compression:     "gzip",
		MaxBackups:      10,
		Encrypt:         true,
	})
	if err == nil || !strings.Contains(err.Error(), "远程节点暂不支持加密备份") {
		t.Fatalf("expected encrypted fixed-node update to be rejected, got %v", err)
	}
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

func TestBackupTaskServiceDeleteKeepsTaskWhenUnscheduleFails(t *testing.T) {
	ctx := context.Background()
	service, targets, tasks := newBackupTaskServiceForTest(t)
	if err := targets.Create(ctx, &model.StorageTarget{Name: "local", Type: "local_disk", Enabled: true, ConfigCiphertext: "ciphertext", ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(ctx, BackupTaskUpsertInput{
		Name: "unschedule-failure", Type: "file", Enabled: true, SourcePath: "/srv/data",
		StorageTargetID: 1, RetentionDays: 7, Compression: "gzip", MaxBackups: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	scheduler := &backupTaskSchedulerStub{removeErr: errors.New("remove failed")}
	service.SetScheduler(scheduler)

	if _, err := service.Delete(ctx, created.ID); err == nil {
		t.Fatal("Delete should fail when the scheduler cannot remove the task")
	} else {
		var appErr *apperror.AppError
		if !errors.As(err, &appErr) || appErr.Code != "BACKUP_TASK_UNSCHEDULE_FAILED" {
			t.Fatalf("Delete error = %#v", err)
		}
	}
	stored, err := tasks.FindByID(ctx, created.ID)
	if err != nil || stored == nil {
		t.Fatalf("task should remain after unschedule failure: task=%#v err=%v", stored, err)
	}
	if len(scheduler.removedIDs) != 1 || len(scheduler.syncedTasks) != 1 {
		t.Fatalf("scheduler calls = remove:%v sync:%d", scheduler.removedIDs, len(scheduler.syncedTasks))
	}
}

func TestBackupTaskServiceDeleteRestoresScheduleWhenPersistenceFails(t *testing.T) {
	ctx := context.Background()
	service, targets, tasks := newBackupTaskServiceForTest(t)
	if err := targets.Create(ctx, &model.StorageTarget{Name: "local", Type: "local_disk", Enabled: true, ConfigCiphertext: "ciphertext", ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatal(err)
	}
	created, err := service.Create(ctx, BackupTaskUpsertInput{
		Name: "delete-failure", Type: "file", Enabled: true, SourcePath: "/srv/data",
		StorageTargetID: 1, RetentionDays: 7, Compression: "gzip", MaxBackups: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	service.tasks = &failingDeleteBackupTaskRepository{
		BackupTaskRepository: tasks,
		deleteErr:            errors.New("database delete failed"),
	}
	scheduler := &backupTaskSchedulerStub{}
	service.SetScheduler(scheduler)

	if _, err := service.Delete(ctx, created.ID); err == nil {
		t.Fatal("Delete should fail when persistence fails")
	} else {
		var appErr *apperror.AppError
		if !errors.As(err, &appErr) || appErr.Code != "BACKUP_TASK_DELETE_FAILED" {
			t.Fatalf("Delete error = %#v", err)
		}
	}
	if len(scheduler.removedIDs) != 1 || len(scheduler.syncedTasks) != 1 || scheduler.syncedTasks[0].ID != created.ID {
		t.Fatalf("scheduler rollback calls = remove:%v sync:%#v", scheduler.removedIDs, scheduler.syncedTasks)
	}
	stored, err := tasks.FindByID(ctx, created.ID)
	if err != nil || stored == nil {
		t.Fatalf("task should remain after persistence failure: task=%#v err=%v", stored, err)
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

func TestSQLServerTaskValidation(t *testing.T) {
	input := BackupTaskUpsertInput{Type: "sqlserver", DBHost: "localhost", DBPort: 1433, DBUser: "backup", DBPassword: "secret", DBName: "app", ExtraConfig: map[string]any{"instanceName": "SQLEXPRESS", "trustServerCertificate": true}}
	if err := validateTaskTypeSpecificFields(input, true); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*BackupTaskUpsertInput){
		func(in *BackupTaskUpsertInput) { in.DBHost = "192.0.2.1" },
		func(in *BackupTaskUpsertInput) { in.NodePoolTag = "any-node" },
		func(in *BackupTaskUpsertInput) { in.VerifyEnabled = true },
		func(in *BackupTaskUpsertInput) { in.DBName = "app,other" },
	} {
		invalid := input
		change(&invalid)
		if err := validateTaskTypeSpecificFields(invalid, true); err == nil {
			t.Fatalf("accepted invalid SQL Server configuration: %+v", invalid)
		}
	}
}
