package retention

import (
	"testing"

	"backupx/server/internal/model"
)

func retentionRecIDs(records []model.BackupRecord) []uint {
	ids := make([]uint, 0, len(records))
	for _, r := range records {
		ids = append(ids, r.ID)
	}
	return ids
}

// 基线全量仍被「不在删除集合中的差异」依赖 → 必须保留，否则差异无法恢复。
func TestProtectDifferentialBasesKeepsBaseWithSurvivingDiff(t *testing.T) {
	all := []model.BackupRecord{
		{ID: 1, BackupKind: model.BackupKindFull},
		{ID: 2, BackupKind: model.BackupKindDifferential, BaseRecordID: 1},
	}
	candidates := []model.BackupRecord{{ID: 1, BackupKind: model.BackupKindFull}}
	if got := protectDifferentialBases(all, candidates); len(got) != 0 {
		t.Fatalf("base with surviving diff must be protected, got %v", retentionRecIDs(got))
	}
}

// 基线全量与其全部差异都在删除集合中 → 可一并删除（无残留差异失去基线）。
func TestProtectDifferentialBasesDeletesBaseWhenDiffAlsoDeleted(t *testing.T) {
	all := []model.BackupRecord{
		{ID: 1, BackupKind: model.BackupKindFull},
		{ID: 2, BackupKind: model.BackupKindDifferential, BaseRecordID: 1},
	}
	candidates := []model.BackupRecord{
		{ID: 1, BackupKind: model.BackupKindFull},
		{ID: 2, BackupKind: model.BackupKindDifferential, BaseRecordID: 1},
	}
	if got := protectDifferentialBases(all, candidates); len(got) != 2 {
		t.Fatalf("base+diff both expired should both be deleted, got %v", retentionRecIDs(got))
	}
}

// 无差异备份时原样透传（不影响既有全量保留逻辑）。
func TestProtectDifferentialBasesNoDiffsPassThrough(t *testing.T) {
	all := []model.BackupRecord{
		{ID: 1, BackupKind: model.BackupKindFull},
		{ID: 2, BackupKind: model.BackupKindFull},
	}
	candidates := []model.BackupRecord{{ID: 1, BackupKind: model.BackupKindFull}}
	got := protectDifferentialBases(all, candidates)
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("no diffs should pass through unchanged, got %v", retentionRecIDs(got))
	}
}
