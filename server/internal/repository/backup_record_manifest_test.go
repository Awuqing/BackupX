package repository

import (
	"context"
	"testing"
	"time"

	"backupx/server/internal/model"
)

// 列表查询应省略 Manifest 列（避免拖出大 JSON），而 FindByID 仍保留（内容浏览/差异基线需要）。
func TestListSuccessfulByTaskOmitsManifest(t *testing.T) {
	ctx := context.Background()
	repo := newBackupRecordTestRepository(t)
	rec := &model.BackupRecord{TaskID: 1, StorageTargetID: 1, Status: "success", BackupKind: model.BackupKindFull, Manifest: `{"entries":[{"p":"x"}]}`, StartedAt: time.Now().UTC()}
	if err := repo.Create(ctx, rec); err != nil {
		t.Fatalf("create: %v", err)
	}

	items, err := repo.ListSuccessfulByTask(ctx, 1)
	if err != nil {
		t.Fatalf("ListSuccessfulByTask: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one record")
	}
	for _, it := range items {
		if it.Manifest != "" {
			t.Fatalf("ListSuccessfulByTask must omit Manifest, got %q", it.Manifest)
		}
	}

	full, err := repo.FindByID(ctx, rec.ID)
	if err != nil || full == nil {
		t.Fatalf("FindByID: %v / %v", full, err)
	}
	if full.Manifest == "" {
		t.Fatal("FindByID must retain Manifest (browse/diff depend on it)")
	}
}

// 仅统计「成功且依赖给定全量」的差异备份：失败的或依赖其他全量的不计入。
func TestCountDependentDifferentials(t *testing.T) {
	ctx := context.Background()
	repo := newBackupRecordTestRepository(t)
	now := time.Now().UTC()
	base := &model.BackupRecord{TaskID: 1, StorageTargetID: 1, Status: "success", BackupKind: model.BackupKindFull, StartedAt: now}
	if err := repo.Create(ctx, base); err != nil {
		t.Fatalf("create base: %v", err)
	}
	mk := func(status, kind string, baseID uint) {
		if err := repo.Create(ctx, &model.BackupRecord{TaskID: 1, StorageTargetID: 1, Status: status, BackupKind: kind, BaseRecordID: baseID, StartedAt: now}); err != nil {
			t.Fatalf("create dependent: %v", err)
		}
	}
	mk(model.BackupRecordStatusSuccess, model.BackupKindDifferential, base.ID) // 计入
	mk(model.BackupRecordStatusSuccess, model.BackupKindDifferential, base.ID) // 计入
	mk(model.BackupRecordStatusFailed, model.BackupKindDifferential, base.ID)  // 失败 → 不计
	mk(model.BackupRecordStatusSuccess, model.BackupKindDifferential, 99999)   // 依赖其他全量 → 不计

	n, err := repo.CountDependentDifferentials(ctx, base.ID)
	if err != nil {
		t.Fatalf("CountDependentDifferentials: %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2 dependent successful differentials, got %d", n)
	}
}
