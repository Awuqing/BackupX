package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"backupx/server/internal/backup"
	"backupx/server/internal/model"
)

func TestMySQLPITRMetadataPersistenceAndChain(t *testing.T) {
	ctx := context.Background()
	execution, _, tasks, _, records, _, _ := newExecutionTestServices(t)
	task, err := tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if task == nil {
		t.Fatal("missing fixture task")
	}
	now := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	task.Type = "mysql"
	task.DBHost = "db"
	task.DBPort = 3306
	task.DBName = "*"
	task.ExtraConfig = `{"mysqlPitr":true}`
	if err := tasks.Update(ctx, task); err != nil {
		t.Fatal(err)
	}
	checkpoint := backup.MySQLCheckpoint{ServerUUID: "uuid", Source: "db:3306", StartFile: "bin.000001", StartPosition: 4, EndFile: "bin.000001", EndPosition: 80, CapturedAt: now, BaselineAt: now, Full: true}
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	full := &model.BackupRecord{TaskID: task.ID, StorageTargetID: task.StorageTargetID, Status: model.BackupRecordStatusRunning, StartedAt: now}
	if err := records.Create(ctx, full); err != nil {
		t.Fatal(err)
	}
	if err := execution.finalizeRecord(ctx, task, full.ID, now, model.BackupRecord{Status: model.BackupRecordStatusSuccess, BackupKind: model.BackupKindFull, StoragePath: "full.pitr.tar", MySQLCheckpoint: string(raw)}); err != nil {
		t.Fatal(err)
	}
	stored, err := records.FindByID(ctx, full.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.MySQLCheckpoint != string(raw) {
		t.Fatal("checkpoint lost in finalization")
	}
	id, base, err := resolveMySQLBase(ctx, records, task, 0, now.Add(time.Hour))
	if err != nil || id != full.ID || base == nil {
		t.Fatalf("base resolution: %d %+v %v", id, base, err)
	}
	if id, _, err := resolveMySQLBase(ctx, records, task, 0, now.Add(25*time.Hour)); err != nil || id != 0 {
		t.Fatalf("old base not rotated: %d %v", id, err)
	}
	checkpoint.Full = false
	checkpoint.EndFile = "bin.000002"
	checkpoint.Files = []string{"bin.000001", "bin.000002"}
	checkpoint.CapturedAt = now.Add(time.Hour)
	raw, err = json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	log := &model.BackupRecord{TaskID: task.ID, StorageTargetID: task.StorageTargetID, Status: model.BackupRecordStatusSuccess, BackupKind: model.BackupKindBinlog, BaseRecordID: full.ID, StoragePath: "logs.pitr.tar", MySQLCheckpoint: string(raw), StartedAt: now.Add(time.Hour)}
	if err := records.Create(ctx, log); err != nil {
		t.Fatal(err)
	}
	service := &RestoreService{records: records}
	chain, err := service.buildRestoreChain(ctx, log)
	if err != nil || len(chain) != 2 || chain[0].ID != full.ID {
		t.Fatalf("chain: %v %v", chain, err)
	}
	if err := execution.DeleteRecord(ctx, full.ID); err == nil {
		t.Fatal("deleted baseline with retained binlogs")
	}
	log.MySQLCheckpoint = strings.ReplaceAll(log.MySQLCheckpoint, "uuid", "other")
	if _, err := service.buildRestoreChain(ctx, log); err == nil {
		t.Fatal("accepted mismatched server identity")
	}
}

func TestMySQLPITRRestoreTimeAndAgentChain(t *testing.T) {
	h := newRestoreTestHarness(t, true)
	ctx := context.Background()
	task, err := h.tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	task.Type = "mysql"
	task.DBHost = "db"
	task.DBPort = 3306
	task.DBName = "*"
	task.ExtraConfig = `{"mysqlPitr":true}`
	if err := h.tasks.Update(ctx, task); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	checkpoint := backup.MySQLCheckpoint{ServerUUID: "uuid", Source: "db:3306", StartFile: "bin.000001", StartPosition: 4, EndFile: "bin.000001", EndPosition: 80, CapturedAt: now, BaselineAt: now, Full: true}
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	full := &model.BackupRecord{TaskID: task.ID, NodeID: task.NodeID, StorageTargetID: task.StorageTargetID, Status: model.BackupRecordStatusSuccess, BackupKind: model.BackupKindFull, StoragePath: "full.pitr.tar.enc", FileName: "full.pitr.tar.enc", MySQLCheckpoint: string(raw), StartedAt: now}
	if err := h.records.Create(ctx, full); err != nil {
		t.Fatal(err)
	}
	checkpoint.Full = false
	checkpoint.EndFile = "bin.000002"
	checkpoint.Files = []string{"bin.000001", "bin.000002"}
	checkpoint.CapturedAt = now.Add(time.Hour)
	raw, err = json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	log := &model.BackupRecord{TaskID: task.ID, NodeID: task.NodeID, StorageTargetID: task.StorageTargetID, Status: model.BackupRecordStatusSuccess, BackupKind: model.BackupKindBinlog, BaseRecordID: full.ID, StoragePath: "logs.pitr.tar.enc", FileName: "logs.pitr.tar.enc", MySQLCheckpoint: string(raw), StartedAt: now.Add(time.Hour)}
	if err := h.records.Create(ctx, log); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"bad", now.Format("2006-01-02T15:04:05"), now.Add(-time.Second).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)} {
		if _, err := h.service.StartSelective(ctx, log.ID, nil, "", "tester", value); err == nil {
			t.Fatalf("accepted invalid restore time %s", value)
		}
	}
	target := now.Add(30 * time.Minute)
	detail, err := h.service.StartSelective(ctx, log.ID, nil, "", "tester", target.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	if detail.RestoreTime == nil || !detail.RestoreTime.Equal(target) {
		t.Fatal("restore time not preserved")
	}
	node, err := h.nodes.FindByID(ctx, task.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := h.service.GetAgentRestoreSpec(ctx, node, detail.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Artifacts) != 2 || spec.Artifacts[0].BackupRecordID != full.ID || spec.Artifacts[1].BackupRecordID != log.ID || spec.Artifacts[0].EncryptionKey == spec.Artifacts[1].EncryptionKey || spec.RestoreTime == nil || !spec.RestoreTime.Equal(target) {
		t.Fatalf("bad restore spec: %+v", spec)
	}
	if _, err := h.service.DownloadAgentArtifact(ctx, node, detail.ID, 999999); err == nil {
		t.Fatal("downloaded object outside restore chain")
	}
}
