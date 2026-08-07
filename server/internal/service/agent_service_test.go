package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
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
	"gorm.io/gorm"
)

func newAgentServicePoolTestHarness(t *testing.T) (*AgentService, *gorm.DB, repository.BackupRecordRepository, repository.AgentCommandRepository, *model.Node, *model.Node) {
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
	targetConfig, err := cipher.EncryptJSON(map[string]any{"basePath": t.TempDir(), "masterRelay": true})
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
	storageRegistry := storage.NewRegistry(storageRclone.NewLocalDiskFactory())
	return NewAgentService(nodeRepo, taskRepo, recordRepo, storageRepo, cmdRepo, cipher, storageRegistry), db, recordRepo, cmdRepo, owner, other
}

func TestAgentServicePooledTaskUsesRecordNodeForSpecAndRecordUpdates(t *testing.T) {
	svc, _, records, _, owner, other := newAgentServicePoolTestHarness(t)
	ctx := context.Background()

	spec, err := svc.GetTaskSpec(ctx, owner, 1)
	if err != nil {
		t.Fatalf("owner GetTaskSpec returned error: %v", err)
	}
	if spec.TaskID != 1 || len(spec.StorageTargets) != 1 {
		t.Fatalf("unexpected spec: %#v", spec)
	}
	if spec.StorageTargets[0].TransferMode != storage.TransferModeMasterRelay {
		t.Fatalf("expected local disk to use Master relay, got %#v", spec.StorageTargets[0])
	}
	if _, err := svc.GetTaskSpec(ctx, other, 1); err == nil {
		t.Fatal("expected non-owner node to be forbidden from pooled task spec")
	}
	record, err := records.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	storagePath := backup.BuildRecordStorageKey("file", record.StartedAt, record.ID, "backup.tar.gz")

	if err := svc.UpdateRecord(ctx, owner, 1, AgentRecordUpdate{
		Status:              model.BackupRecordStatusSuccess,
		FileName:            "backup.tar.gz",
		FileSize:            123,
		StoragePath:         storagePath,
		StorageTargetID:     1,
		StorageTransferMode: storage.TransferModeMasterRelay,
		StorageUploadResults: []StorageUploadResultItem{
			{StorageTargetID: 1, StorageTargetName: "local", Status: "success", StoragePath: storagePath, FileSize: 123, TransferMode: storage.TransferModeMasterRelay},
		},
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
	if updated.StorageTargetID != 1 {
		t.Fatalf("expected successful storage target id 1, got %d", updated.StorageTargetID)
	}
	if updated.StorageTransferMode != storage.TransferModeMasterRelay {
		t.Fatalf("expected Master relay transfer mode, got %q", updated.StorageTransferMode)
	}
	if !strings.Contains(updated.StorageUploadResults, `"storageTargetName":"local"`) {
		t.Fatalf("expected upload results to be persisted, got %q", updated.StorageUploadResults)
	}
	if err := svc.UpdateRecord(ctx, other, 1, AgentRecordUpdate{LogAppend: "bad"}); err == nil {
		t.Fatal("expected non-owner node to be forbidden from record update")
	}
}

func TestAgentServiceRelaysRemoteArtifactToMasterLocalDisk(t *testing.T) {
	svc, _, records, _, owner, other := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	payload := []byte("artifact from remote source server")
	digest := sha256.Sum256(payload)
	checksum := fmt.Sprintf("%x", digest[:])
	record, err := records.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	objectKey := backup.BuildRecordStorageKey("file", record.StartedAt, record.ID, "remote-source.tar")

	if err := svc.UploadArtifact(ctx, owner, 1, 1, objectKey, int64(len(payload)), checksum, bytes.NewReader(payload)); err != nil {
		t.Fatalf("UploadArtifact returned error: %v", err)
	}
	target, err := svc.storageRepo.FindByID(ctx, 1)
	if err != nil || target == nil {
		t.Fatalf("FindByID target: target=%#v err=%v", target, err)
	}
	config := map[string]any{}
	if err := svc.cipher.DecryptJSON(target.ConfigCiphertext, &config); err != nil {
		t.Fatalf("DecryptJSON target config: %v", err)
	}
	basePath, _ := config["basePath"].(string)
	stored, err := os.ReadFile(filepath.Join(basePath, filepath.FromSlash(objectKey)))
	if err != nil {
		t.Fatalf("read relayed artifact: %v", err)
	}
	if !bytes.Equal(stored, payload) {
		t.Fatalf("relayed artifact differs: got %q", stored)
	}
	if err := svc.UploadArtifact(ctx, other, 1, 1, objectKey, int64(len(payload)), checksum, bytes.NewReader(payload)); err == nil {
		t.Fatal("expected a different node to be forbidden from relaying the artifact")
	}

	legacyPayload := []byte("artifact from an older Agent")
	legacyDigest := sha256.Sum256(legacyPayload)
	legacyKey := backup.BuildStorageKey("file", record.StartedAt, "legacy-agent.tar")
	canonicalKey := backup.BuildRecordStorageKey("file", record.StartedAt, record.ID, "legacy-agent.tar")
	if err := svc.UploadArtifact(ctx, owner, record.ID, target.ID, legacyKey, int64(len(legacyPayload)), fmt.Sprintf("%x", legacyDigest[:]), bytes.NewReader(legacyPayload)); err != nil {
		t.Fatalf("UploadArtifact legacy key returned error: %v", err)
	}
	stored, err = os.ReadFile(filepath.Join(basePath, filepath.FromSlash(canonicalKey)))
	if err != nil || !bytes.Equal(stored, legacyPayload) {
		t.Fatalf("legacy Agent artifact was not normalized: data=%q err=%v", stored, err)
	}
	if _, err := os.Stat(filepath.Join(basePath, filepath.FromSlash(legacyKey))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy object key should not be written directly: %v", err)
	}
	if err := svc.UpdateRecord(ctx, owner, record.ID, AgentRecordUpdate{
		Status:          model.BackupRecordStatusSuccess,
		FileName:        "legacy-agent.tar",
		FileSize:        int64(len(legacyPayload)),
		Checksum:        fmt.Sprintf("%x", legacyDigest[:]),
		StoragePath:     legacyKey,
		StorageTargetID: target.ID,
		StorageUploadResults: []StorageUploadResultItem{{
			StorageTargetID: target.ID,
			Status:          "success",
			StoragePath:     legacyKey,
			FileSize:        int64(len(legacyPayload)),
		}},
	}); err != nil {
		t.Fatalf("UpdateRecord legacy key returned error: %v", err)
	}
	updated, err := records.FindByID(ctx, record.ID)
	if err != nil {
		t.Fatalf("FindByID updated record returned error: %v", err)
	}
	if updated.StoragePath != canonicalKey || updated.StorageTransferMode != storage.TransferModeMasterRelay || !strings.Contains(updated.StorageUploadResults, canonicalKey) {
		t.Fatalf("legacy Agent record was not normalized: %#v", updated)
	}
}

func TestAgentServiceRejectsArtifactOutsideRecordNamespace(t *testing.T) {
	svc, _, records, _, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	record, err := records.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	target, err := svc.storageRepo.FindByID(ctx, 1)
	if err != nil || target == nil {
		t.Fatalf("FindByID target: target=%#v err=%v", target, err)
	}
	config := map[string]any{}
	if err := svc.cipher.DecryptJSON(target.ConfigCiphertext, &config); err != nil {
		t.Fatalf("DecryptJSON target config: %v", err)
	}
	basePath, _ := config["basePath"].(string)
	victimKey := backup.BuildRecordStorageKey("file", record.StartedAt, record.ID+1, "victim.tar")
	victimPath := filepath.Join(basePath, filepath.FromSlash(victimKey))
	if err := os.MkdirAll(filepath.Dir(victimPath), 0o755); err != nil {
		t.Fatalf("MkdirAll victim parent: %v", err)
	}
	if err := os.WriteFile(victimPath, []byte("keep me"), 0o600); err != nil {
		t.Fatalf("WriteFile victim: %v", err)
	}
	payload := []byte("overwrite")
	digest := sha256.Sum256(payload)
	if err := svc.UploadArtifact(ctx, owner, record.ID, target.ID, victimKey, int64(len(payload)), fmt.Sprintf("%x", digest[:]), bytes.NewReader(payload)); err == nil {
		t.Fatal("expected another record namespace to be rejected")
	}
	stored, err := os.ReadFile(victimPath)
	if err != nil {
		t.Fatalf("ReadFile victim: %v", err)
	}
	if string(stored) != "keep me" {
		t.Fatalf("victim object changed: %q", stored)
	}
	if err := svc.UpdateRecord(ctx, owner, record.ID, AgentRecordUpdate{StoragePath: victimKey, StorageTargetID: target.ID, StorageTransferMode: storage.TransferModeMasterRelay}); err == nil {
		t.Fatal("expected another record namespace in status update to be rejected")
	}
}

func TestAgentServiceKeepsExistingLocalDiskTargetsAgentLocal(t *testing.T) {
	svc, _, records, _, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	target, err := svc.storageRepo.FindByID(ctx, 1)
	if err != nil || target == nil {
		t.Fatalf("FindByID target: target=%#v err=%v", target, err)
	}
	legacyConfig, err := svc.cipher.EncryptJSON(map[string]any{"basePath": t.TempDir()})
	if err != nil {
		t.Fatalf("EncryptJSON legacy target: %v", err)
	}
	target.ConfigCiphertext = legacyConfig
	if err := svc.storageRepo.Update(ctx, target); err != nil {
		t.Fatalf("Update legacy target: %v", err)
	}

	spec, err := svc.GetTaskSpec(ctx, owner, 1)
	if err != nil {
		t.Fatalf("GetTaskSpec returned error: %v", err)
	}
	if len(spec.StorageTargets) != 1 || spec.StorageTargets[0].TransferMode != storage.TransferModeDirect {
		t.Fatalf("expected legacy local disk to stay Agent-local, got %#v", spec.StorageTargets)
	}
	payload := []byte("must not be relayed")
	digest := sha256.Sum256(payload)
	record, findErr := records.FindByID(ctx, 1)
	if findErr != nil {
		t.Fatalf("FindByID record returned error: %v", findErr)
	}
	objectKey := backup.BuildRecordStorageKey("file", record.StartedAt, record.ID, "legacy.tar")
	err = svc.UploadArtifact(ctx, owner, 1, 1, objectKey, int64(len(payload)), fmt.Sprintf("%x", digest[:]), bytes.NewReader(payload))
	if err == nil {
		t.Fatal("expected relay upload to be rejected for an Agent-local target")
	}
}

func TestAgentServiceUpdateRecordRefreshesTaskSummaryOnTerminalStatus(t *testing.T) {
	for _, status := range []string{model.BackupRecordStatusSuccess, model.BackupRecordStatusFailed} {
		t.Run(status, func(t *testing.T) {
			svc, _, records, _, owner, _ := newAgentServicePoolTestHarness(t)
			ctx := context.Background()
			record, err := records.FindByID(ctx, 1)
			if err != nil {
				t.Fatalf("FindByID record returned error: %v", err)
			}

			if err := svc.UpdateRecord(ctx, owner, record.ID, AgentRecordUpdate{Status: status}); err != nil {
				t.Fatalf("UpdateRecord returned error: %v", err)
			}

			task, err := svc.taskRepo.FindByID(ctx, record.TaskID)
			if err != nil {
				t.Fatalf("FindByID task returned error: %v", err)
			}
			if task.LastStatus != status {
				t.Fatalf("expected task LastStatus %q, got %q", status, task.LastStatus)
			}
			if task.LastRunAt == nil || !task.LastRunAt.Equal(record.StartedAt) {
				t.Fatalf("expected task LastRunAt to match record startedAt %s, got %#v", record.StartedAt, task.LastRunAt)
			}
		})
	}
}

func TestAgentServiceUpdateRecordReturnsTaskSummaryUpdateError(t *testing.T) {
	svc, _, _, _, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	expectedErr := errors.New("task update failed")
	svc.taskRepo = &failingUpdateTaskRepo{
		BackupTaskRepository: svc.taskRepo,
		err:                  expectedErr,
	}

	err := svc.UpdateRecord(ctx, owner, 1, AgentRecordUpdate{Status: model.BackupRecordStatusSuccess})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected task update error %v, got %v", expectedErr, err)
	}
}

func TestAgentServiceProcessStaleCommandsFailsPendingRunTaskRecord(t *testing.T) {
	svc, _, records, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	oldCommand := &model.AgentCommand{
		NodeID:    owner.ID,
		Type:      model.AgentCommandTypeRunTask,
		Status:    model.AgentCommandStatusPending,
		Payload:   `{"recordId":1}`,
		CreatedAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := commands.Create(ctx, oldCommand); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	svc.processStaleCommands(ctx, time.Now().UTC().Add(-30*time.Minute))

	updatedCommand, err := commands.FindByID(ctx, oldCommand.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusTimeout {
		t.Fatalf("expected command timeout, got %#v", updatedCommand)
	}
	updatedRecord, err := records.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	if updatedRecord.Status != model.BackupRecordStatusFailed {
		t.Fatalf("expected record failed, got %#v", updatedRecord)
	}
	if updatedRecord.CompletedAt == nil {
		t.Fatal("expected failed record completedAt to be set")
	}
}

func TestAgentServiceProcessStaleCommandsFailsPendingRestoreRecord(t *testing.T) {
	svc, db, _, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	restoreRepo := repository.NewRestoreRecordRepository(db)
	restore := &model.RestoreRecord{
		BackupRecordID: 1,
		TaskID:         1,
		NodeID:         owner.ID,
		Status:         model.RestoreRecordStatusRunning,
		StartedAt:      time.Now().UTC().Add(-time.Hour),
	}
	if err := restoreRepo.Create(ctx, restore); err != nil {
		t.Fatalf("Create restore returned error: %v", err)
	}
	svc.SetRestoreRepository(restoreRepo)
	oldCommand := &model.AgentCommand{
		NodeID:    owner.ID,
		Type:      model.AgentCommandTypeRestoreRecord,
		Status:    model.AgentCommandStatusPending,
		Payload:   `{"restoreRecordId":1}`,
		CreatedAt: time.Now().UTC().Add(-time.Hour),
	}
	if err := commands.Create(ctx, oldCommand); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	svc.processStaleCommands(ctx, time.Now().UTC().Add(-30*time.Minute))

	updatedCommand, err := commands.FindByID(ctx, oldCommand.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusTimeout {
		t.Fatalf("expected command timeout, got %#v", updatedCommand)
	}
	updatedRestore, err := restoreRepo.FindByID(ctx, restore.ID)
	if err != nil {
		t.Fatalf("FindByID restore returned error: %v", err)
	}
	if updatedRestore.Status != model.RestoreRecordStatusFailed {
		t.Fatalf("expected restore failed, got %#v", updatedRestore)
	}
	if updatedRestore.CompletedAt == nil {
		t.Fatal("expected failed restore completedAt to be set")
	}
}

func TestAgentServiceProcessStaleCommandsKeepsActiveDispatchedRunTaskRecord(t *testing.T) {
	svc, _, records, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	dispatchedAt := time.Now().UTC().Add(-time.Hour)
	oldCommand := &model.AgentCommand{
		NodeID:       owner.ID,
		Type:         model.AgentCommandTypeRunTask,
		Status:       model.AgentCommandStatusDispatched,
		Payload:      `{"recordId":1}`,
		CreatedAt:    dispatchedAt,
		DispatchedAt: &dispatchedAt,
	}
	if err := commands.Create(ctx, oldCommand); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	svc.processStaleCommands(ctx, time.Now().UTC().Add(-30*time.Minute))

	updatedCommand, err := commands.FindByID(ctx, oldCommand.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusDispatched {
		t.Fatalf("expected active command to remain dispatched, got %#v", updatedCommand)
	}
	updatedRecord, err := records.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	if updatedRecord.Status != model.BackupRecordStatusRunning {
		t.Fatalf("expected active record to remain running, got %#v", updatedRecord)
	}
}

func TestAgentServiceProcessStaleCommandsKeepsDispatchedRunTaskWhenNodeHeartbeatIsFresh(t *testing.T) {
	svc, db, records, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	dispatchedAt := time.Now().UTC().Add(-time.Hour)
	if err := setBackupRecordUpdatedAt(db, 1, dispatchedAt); err != nil {
		t.Fatalf("set backup record updated_at: %v", err)
	}
	if err := db.Model(&model.Node{}).Where("id = ?", owner.ID).UpdateColumn("last_seen", time.Now().UTC()).Error; err != nil {
		t.Fatalf("set owner last_seen: %v", err)
	}
	oldCommand := &model.AgentCommand{
		NodeID:       owner.ID,
		Type:         model.AgentCommandTypeRunTask,
		Status:       model.AgentCommandStatusDispatched,
		Payload:      `{"recordId":1}`,
		CreatedAt:    dispatchedAt,
		DispatchedAt: &dispatchedAt,
	}
	if err := commands.Create(ctx, oldCommand); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	svc.processStaleCommands(ctx, time.Now().UTC().Add(-30*time.Minute))

	updatedCommand, err := commands.FindByID(ctx, oldCommand.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusDispatched {
		t.Fatalf("expected command to remain dispatched while node heartbeat is fresh, got %#v", updatedCommand)
	}
	updatedRecord, err := records.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	if updatedRecord.Status != model.BackupRecordStatusRunning {
		t.Fatalf("expected record to remain running while node heartbeat is fresh, got %#v", updatedRecord)
	}
}

func TestAgentServiceProcessStaleCommandsTimesOutShortCommandEvenWhenNodeHeartbeatIsFresh(t *testing.T) {
	svc, db, _, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	dispatchedAt := time.Now().UTC().Add(-time.Hour)
	if err := db.Model(&model.Node{}).Where("id = ?", owner.ID).UpdateColumn("last_seen", time.Now().UTC()).Error; err != nil {
		t.Fatalf("set owner last_seen: %v", err)
	}
	oldCommand := &model.AgentCommand{
		NodeID:       owner.ID,
		Type:         model.AgentCommandTypeListDir,
		Status:       model.AgentCommandStatusDispatched,
		Payload:      `{"path":"/srv"}`,
		CreatedAt:    dispatchedAt,
		DispatchedAt: &dispatchedAt,
	}
	if err := commands.Create(ctx, oldCommand); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	svc.processStaleCommands(ctx, time.Now().UTC().Add(-30*time.Minute))

	updatedCommand, err := commands.FindByID(ctx, oldCommand.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusTimeout {
		t.Fatalf("expected stale short command timeout, got %#v", updatedCommand)
	}
}

func TestAgentServiceProcessStaleCommandsTimesOutDispatchedRunTaskWhenRecordIsTerminalEvenWithFreshHeartbeat(t *testing.T) {
	svc, db, records, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	dispatchedAt := time.Now().UTC().Add(-time.Hour)
	if err := db.Model(&model.Node{}).Where("id = ?", owner.ID).UpdateColumn("last_seen", time.Now().UTC()).Error; err != nil {
		t.Fatalf("set owner last_seen: %v", err)
	}
	record, err := records.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	completedAt := time.Now().UTC().Add(-time.Minute)
	record.Status = model.BackupRecordStatusFailed
	record.CompletedAt = &completedAt
	if err := records.Update(ctx, record); err != nil {
		t.Fatalf("Update terminal record returned error: %v", err)
	}
	oldCommand := &model.AgentCommand{
		NodeID:       owner.ID,
		Type:         model.AgentCommandTypeRunTask,
		Status:       model.AgentCommandStatusDispatched,
		Payload:      `{"recordId":1}`,
		CreatedAt:    dispatchedAt,
		DispatchedAt: &dispatchedAt,
	}
	if err := commands.Create(ctx, oldCommand); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	svc.processStaleCommands(ctx, time.Now().UTC().Add(-30*time.Minute))

	updatedCommand, err := commands.FindByID(ctx, oldCommand.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusTimeout {
		t.Fatalf("expected command timeout when linked record is terminal, got %#v", updatedCommand)
	}
}

func TestAgentServiceProcessStaleCommandsTimesOutInactiveDispatchedRunTaskRecord(t *testing.T) {
	svc, db, records, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	dispatchedAt := time.Now().UTC().Add(-time.Hour)
	if err := setBackupRecordUpdatedAt(db, 1, dispatchedAt); err != nil {
		t.Fatalf("set backup record updated_at: %v", err)
	}
	if err := db.Model(&model.Node{}).Where("id = ?", owner.ID).UpdateColumn("last_seen", dispatchedAt).Error; err != nil {
		t.Fatalf("set owner last_seen: %v", err)
	}
	oldCommand := &model.AgentCommand{
		NodeID:       owner.ID,
		Type:         model.AgentCommandTypeRunTask,
		Status:       model.AgentCommandStatusDispatched,
		Payload:      `{"recordId":1}`,
		CreatedAt:    dispatchedAt,
		DispatchedAt: &dispatchedAt,
	}
	if err := commands.Create(ctx, oldCommand); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	svc.processStaleCommands(ctx, time.Now().UTC().Add(-30*time.Minute))

	updatedCommand, err := commands.FindByID(ctx, oldCommand.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusTimeout {
		t.Fatalf("expected inactive command timeout, got %#v", updatedCommand)
	}
	updatedRecord, err := records.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	if updatedRecord.Status != model.BackupRecordStatusFailed {
		t.Fatalf("expected inactive record failed, got %#v", updatedRecord)
	}
}

func TestAgentServiceProcessStaleCommandsKeepsActiveDispatchedRestoreRecord(t *testing.T) {
	svc, db, _, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	restoreRepo := repository.NewRestoreRecordRepository(db)
	restore := createAgentServiceRestoreRecord(t, restoreRepo, owner.ID)
	svc.SetRestoreRepository(restoreRepo)
	dispatchedAt := time.Now().UTC().Add(-time.Hour)
	oldCommand := &model.AgentCommand{
		NodeID:       owner.ID,
		Type:         model.AgentCommandTypeRestoreRecord,
		Status:       model.AgentCommandStatusDispatched,
		Payload:      `{"restoreRecordId":1}`,
		CreatedAt:    dispatchedAt,
		DispatchedAt: &dispatchedAt,
	}
	if err := commands.Create(ctx, oldCommand); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	svc.processStaleCommands(ctx, time.Now().UTC().Add(-30*time.Minute))

	updatedCommand, err := commands.FindByID(ctx, oldCommand.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusDispatched {
		t.Fatalf("expected active restore command to remain dispatched, got %#v", updatedCommand)
	}
	updatedRestore, err := restoreRepo.FindByID(ctx, restore.ID)
	if err != nil {
		t.Fatalf("FindByID restore returned error: %v", err)
	}
	if updatedRestore.Status != model.RestoreRecordStatusRunning {
		t.Fatalf("expected active restore to remain running, got %#v", updatedRestore)
	}
}

func TestAgentServiceProcessStaleCommandsKeepsDispatchedRestoreWhenNodeHeartbeatIsFresh(t *testing.T) {
	svc, db, _, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	restoreRepo := repository.NewRestoreRecordRepository(db)
	restore := createAgentServiceRestoreRecord(t, restoreRepo, owner.ID)
	svc.SetRestoreRepository(restoreRepo)
	dispatchedAt := time.Now().UTC().Add(-time.Hour)
	if err := setRestoreRecordUpdatedAt(db, restore.ID, dispatchedAt); err != nil {
		t.Fatalf("set restore record updated_at: %v", err)
	}
	if err := db.Model(&model.Node{}).Where("id = ?", owner.ID).UpdateColumn("last_seen", time.Now().UTC()).Error; err != nil {
		t.Fatalf("set owner last_seen: %v", err)
	}
	oldCommand := &model.AgentCommand{
		NodeID:       owner.ID,
		Type:         model.AgentCommandTypeRestoreRecord,
		Status:       model.AgentCommandStatusDispatched,
		Payload:      `{"restoreRecordId":1}`,
		CreatedAt:    dispatchedAt,
		DispatchedAt: &dispatchedAt,
	}
	if err := commands.Create(ctx, oldCommand); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	svc.processStaleCommands(ctx, time.Now().UTC().Add(-30*time.Minute))

	updatedCommand, err := commands.FindByID(ctx, oldCommand.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusDispatched {
		t.Fatalf("expected restore command to remain dispatched while node heartbeat is fresh, got %#v", updatedCommand)
	}
}

func TestAgentServiceProcessStaleCommandsTimesOutInactiveDispatchedRestoreRecord(t *testing.T) {
	svc, db, _, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	restoreRepo := repository.NewRestoreRecordRepository(db)
	restore := createAgentServiceRestoreRecord(t, restoreRepo, owner.ID)
	svc.SetRestoreRepository(restoreRepo)
	dispatchedAt := time.Now().UTC().Add(-time.Hour)
	if err := setRestoreRecordUpdatedAt(db, restore.ID, dispatchedAt); err != nil {
		t.Fatalf("set restore record updated_at: %v", err)
	}
	if err := db.Model(&model.Node{}).Where("id = ?", owner.ID).UpdateColumn("last_seen", dispatchedAt).Error; err != nil {
		t.Fatalf("set owner last_seen: %v", err)
	}
	oldCommand := &model.AgentCommand{
		NodeID:       owner.ID,
		Type:         model.AgentCommandTypeRestoreRecord,
		Status:       model.AgentCommandStatusDispatched,
		Payload:      `{"restoreRecordId":1}`,
		CreatedAt:    dispatchedAt,
		DispatchedAt: &dispatchedAt,
	}
	if err := commands.Create(ctx, oldCommand); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	svc.processStaleCommands(ctx, time.Now().UTC().Add(-30*time.Minute))

	updatedCommand, err := commands.FindByID(ctx, oldCommand.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusTimeout {
		t.Fatalf("expected inactive restore command timeout, got %#v", updatedCommand)
	}
	updatedRestore, err := restoreRepo.FindByID(ctx, restore.ID)
	if err != nil {
		t.Fatalf("FindByID restore returned error: %v", err)
	}
	if updatedRestore.Status != model.RestoreRecordStatusFailed {
		t.Fatalf("expected inactive restore failed, got %#v", updatedRestore)
	}
}

func TestAgentServiceSubmitCommandResultDoesNotOverwriteTerminalCommand(t *testing.T) {
	svc, _, _, commands, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	completedAt := time.Now().UTC().Add(-time.Minute)
	command := &model.AgentCommand{
		NodeID:       owner.ID,
		Type:         model.AgentCommandTypeRunTask,
		Status:       model.AgentCommandStatusTimeout,
		Payload:      `{"recordId":1}`,
		ErrorMessage: "timeout",
		CompletedAt:  &completedAt,
	}
	if err := commands.Create(ctx, command); err != nil {
		t.Fatalf("Create command returned error: %v", err)
	}

	if err := svc.SubmitCommandResult(ctx, owner, command.ID, AgentCommandResult{Success: true, Result: []byte(`{"ok":true}`)}); err != nil {
		t.Fatalf("SubmitCommandResult returned error: %v", err)
	}

	updatedCommand, err := commands.FindByID(ctx, command.ID)
	if err != nil {
		t.Fatalf("FindByID command returned error: %v", err)
	}
	if updatedCommand.Status != model.AgentCommandStatusTimeout {
		t.Fatalf("expected terminal command status to remain timeout, got %#v", updatedCommand)
	}
	if updatedCommand.Result != "" {
		t.Fatalf("expected terminal command result to remain empty, got %q", updatedCommand.Result)
	}
}

func TestAgentServiceUpdateRecordDoesNotOverwriteTerminalRecord(t *testing.T) {
	svc, _, records, _, owner, _ := newAgentServicePoolTestHarness(t)
	ctx := context.Background()
	record, err := records.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	completedAt := time.Now().UTC().Add(-time.Minute)
	record.Status = model.BackupRecordStatusFailed
	record.ErrorMessage = "timeout"
	record.CompletedAt = &completedAt
	if err := records.Update(ctx, record); err != nil {
		t.Fatalf("Update record returned error: %v", err)
	}

	if err := svc.UpdateRecord(ctx, owner, record.ID, AgentRecordUpdate{
		Status:       model.BackupRecordStatusSuccess,
		FileName:     "late.tar.gz",
		FileSize:     42,
		Checksum:     "late",
		StoragePath:  "late/path",
		ErrorMessage: "late success",
		LogAppend:    "late log\n",
	}); err != nil {
		t.Fatalf("UpdateRecord returned error: %v", err)
	}

	updatedRecord, err := records.FindByID(ctx, record.ID)
	if err != nil {
		t.Fatalf("FindByID updated record returned error: %v", err)
	}
	if updatedRecord.Status != model.BackupRecordStatusFailed {
		t.Fatalf("expected terminal record status to remain failed, got %#v", updatedRecord)
	}
	if updatedRecord.FileName != "" || updatedRecord.StoragePath != "" || updatedRecord.ErrorMessage != "timeout" {
		t.Fatalf("expected terminal record fields to remain unchanged, got %#v", updatedRecord)
	}
}

func createAgentServiceRestoreRecord(t *testing.T, repo repository.RestoreRecordRepository, nodeID uint) *model.RestoreRecord {
	t.Helper()
	restore := &model.RestoreRecord{
		BackupRecordID: 1,
		TaskID:         1,
		NodeID:         nodeID,
		Status:         model.RestoreRecordStatusRunning,
		StartedAt:      time.Now().UTC().Add(-time.Hour),
	}
	if err := repo.Create(context.Background(), restore); err != nil {
		t.Fatalf("Create restore returned error: %v", err)
	}
	return restore
}

func setBackupRecordUpdatedAt(db *gorm.DB, id uint, updatedAt time.Time) error {
	return db.Model(&model.BackupRecord{}).Where("id = ?", id).UpdateColumn("updated_at", updatedAt).Error
}

func setRestoreRecordUpdatedAt(db *gorm.DB, id uint, updatedAt time.Time) error {
	return db.Model(&model.RestoreRecord{}).Where("id = ?", id).UpdateColumn("updated_at", updatedAt).Error
}

type failingUpdateTaskRepo struct {
	repository.BackupTaskRepository
	err error
}

func (r *failingUpdateTaskRepo) Update(context.Context, *model.BackupTask) error {
	return r.err
}
