package service

import (
	"context"
	"os"
	"path/filepath"
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

func newReportTestHarness(t *testing.T) (*ReportService, *BackupExecutionService) {
	t.Helper()
	baseDir := t.TempDir()
	sourceDir := filepath.Join(baseDir, "data")
	storeDir := filepath.Join(baseDir, "store")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "f.txt"), []byte("report-data"), 0o644); err != nil {
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
	cipher := codec.NewConfigCipher("report-secret")
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
	task := &model.BackupTask{Name: "rep-task", Type: "file", Enabled: true, SourcePath: sourceDir, StorageTargetID: 1, NodeID: 0, RetentionDays: 30, Compression: "gzip", MaxBackups: 10, LastStatus: "idle"}
	if err := tasks.Create(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	runnerRegistry := backup.NewRegistry(backup.NewFileRunner(), backup.NewSQLiteRunner(), backup.NewMySQLRunner(nil), backup.NewPostgreSQLRunner(nil))
	storageRegistry := storage.NewRegistry(storageRclone.NewLocalDiskFactory())
	execution := NewBackupExecutionService(tasks, records, targets, storageRegistry, runnerRegistry, backup.NewLogHub(), nil, cipher, nil, baseDir, 2, 10, "")
	return NewReportService(tasks, records), execution
}

func findRow(rows []ComplianceTaskRow, taskID uint) *ComplianceTaskRow {
	for i := range rows {
		if rows[i].TaskID == taskID {
			return &rows[i]
		}
	}
	return nil
}

func TestComplianceReport_ReflectsBackupOutcome(t *testing.T) {
	report, execution := newReportTestHarness(t)
	ctx := context.Background()

	// 备份前：任务启用但从未成功 → at_risk。
	before, err := report.ComplianceReport(ctx, 30)
	if err != nil {
		t.Fatalf("ComplianceReport: %v", err)
	}
	row := findRow(before.Tasks, 1)
	if row == nil {
		t.Fatal("task row missing before backup")
	}
	if row.Risk != "at_risk" || row.Compliant {
		t.Fatalf("expected at_risk before any success, got risk=%s compliant=%v", row.Risk, row.Compliant)
	}
	if before.Summary.AtRiskTasks != 1 || before.Summary.CompliantTasks != 0 {
		t.Fatalf("unexpected summary before: %+v", before.Summary)
	}

	// 跑一次成功备份。
	bd, err := execution.RunTaskByIDSync(ctx, 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync: %v", err)
	}
	if bd.Status != "success" {
		t.Fatalf("backup not success: %s", bd.Status)
	}

	// 备份后：合规、成功率 1.0、保护字节数 > 0。
	after, err := report.ComplianceReport(ctx, 30)
	if err != nil {
		t.Fatalf("ComplianceReport after: %v", err)
	}
	row = findRow(after.Tasks, 1)
	if row == nil {
		t.Fatal("task row missing after backup")
	}
	if !row.Compliant || row.Risk != "ok" {
		t.Fatalf("expected ok/compliant after success, got risk=%s compliant=%v", row.Risk, row.Compliant)
	}
	if row.TotalRuns != 1 || row.Successes != 1 || row.Failures != 0 {
		t.Fatalf("unexpected counts: runs=%d ok=%d fail=%d", row.TotalRuns, row.Successes, row.Failures)
	}
	if row.SuccessRate != 1 {
		t.Fatalf("expected success rate 1.0, got %v", row.SuccessRate)
	}
	if row.LastStatus != "success" || row.LastSuccessAt == nil || row.ProtectedBytes <= 0 {
		t.Fatalf("unexpected last/protected: status=%s lastSuccess=%v bytes=%d", row.LastStatus, row.LastSuccessAt, row.ProtectedBytes)
	}
	if after.Summary.CompliantTasks != 1 || after.Summary.AtRiskTasks != 0 || after.Summary.OverallSuccessRate != 1 {
		t.Fatalf("unexpected summary after: %+v", after.Summary)
	}
	if after.Summary.TotalProtectedB <= 0 {
		t.Fatalf("expected protected bytes > 0, got %d", after.Summary.TotalProtectedB)
	}
}

func TestComplianceReport_RejectsInvalidRange(t *testing.T) {
	report, _ := newReportTestHarness(t)
	ctx := context.Background()
	if _, err := report.ComplianceReport(ctx, 0); err == nil {
		t.Fatal("expected error for days=0")
	}
	if _, err := report.ComplianceReport(ctx, 9999); err == nil {
		t.Fatal("expected error for days>365")
	}
}
