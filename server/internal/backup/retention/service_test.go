package retention

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
)

type fakeRecordRepository struct {
	records    []model.BackupRecord
	deleted    []uint
	deleteErrs map[uint]error
}

func (r *fakeRecordRepository) List(context.Context, repository.BackupRecordListOptions) ([]model.BackupRecord, error) {
	return nil, nil
}
func (r *fakeRecordRepository) FindByID(context.Context, uint) (*model.BackupRecord, error) {
	return nil, nil
}
func (r *fakeRecordRepository) FindRunningByTaskAndNode(context.Context, uint, uint) (*model.BackupRecord, error) {
	return nil, nil
}
func (r *fakeRecordRepository) Create(context.Context, *model.BackupRecord) error { return nil }
func (r *fakeRecordRepository) Update(context.Context, *model.BackupRecord) error { return nil }
func (r *fakeRecordRepository) Delete(_ context.Context, id uint) error {
	if err := r.deleteErrs[id]; err != nil {
		return err
	}
	r.deleted = append(r.deleted, id)
	return nil
}
func (r *fakeRecordRepository) ListRecent(context.Context, int) ([]model.BackupRecord, error) {
	return nil, nil
}
func (r *fakeRecordRepository) ListByTask(_ context.Context, _ uint) ([]model.BackupRecord, error) {
	return r.records, nil
}
func (r *fakeRecordRepository) ListSuccessfulByTask(_ context.Context, _ uint) ([]model.BackupRecord, error) {
	return r.records, nil
}
func (r *fakeRecordRepository) Count(context.Context) (int64, error)                 { return 0, nil }
func (r *fakeRecordRepository) CountSince(context.Context, time.Time) (int64, error) { return 0, nil }
func (r *fakeRecordRepository) CountSuccessSince(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *fakeRecordRepository) SumFileSize(context.Context) (int64, error) { return 0, nil }
func (r *fakeRecordRepository) TimelineSince(context.Context, time.Time) ([]repository.BackupTimelinePoint, error) {
	return nil, nil
}
func (r *fakeRecordRepository) StorageUsage(context.Context) ([]repository.BackupStorageUsageItem, error) {
	return nil, nil
}

type fakeProvider struct {
	deleted []string
	failKey string
}

func (p *fakeProvider) Type() string                         { return storage.ProviderTypeLocalDisk }
func (p *fakeProvider) TestConnection(context.Context) error { return nil }
func (p *fakeProvider) Upload(context.Context, string, io.Reader, int64, map[string]string) error {
	return nil
}
func (p *fakeProvider) Download(context.Context, string) (io.ReadCloser, error) { return nil, nil }
func (p *fakeProvider) Delete(_ context.Context, objectKey string) error {
	if objectKey == p.failKey {
		return fmt.Errorf("delete failed")
	}
	p.deleted = append(p.deleted, objectKey)
	return nil
}
func (p *fakeProvider) List(context.Context, string) ([]storage.ObjectInfo, error) { return nil, nil }

func TestSelectRecordsToDelete(t *testing.T) {
	now := time.Date(2026, 3, 7, 16, 0, 0, 0, time.UTC)
	completedNew := now.Add(-24 * time.Hour)
	completedOld := now.Add(-15 * 24 * time.Hour)
	records := []model.BackupRecord{
		{ID: 3, CompletedAt: &completedNew},
		{ID: 2, CompletedAt: &completedNew},
		{ID: 1, CompletedAt: &completedOld},
	}
	selected := selectRecordsToDelete(records, 7, 2, now)
	if len(selected) != 1 || selected[0].ID != 1 {
		t.Fatalf("unexpected selected records: %#v", selected)
	}
}

func gfsRecord(id uint, ts time.Time, locked bool) model.BackupRecord {
	completed := ts
	return model.BackupRecord{ID: id, StartedAt: ts, CompletedAt: &completed, Locked: locked}
}

func gfsDay(y, m, d, h int) time.Time {
	return time.Date(y, time.Month(m), d, h, 0, 0, 0, time.UTC)
}

func deletedIDSet(records []model.BackupRecord) map[uint]bool {
	out := make(map[uint]bool, len(records))
	for i := range records {
		out[records[i].ID] = true
	}
	return out
}

func assertDeleted(t *testing.T, del []model.BackupRecord, want ...uint) {
	t.Helper()
	got := deletedIDSet(del)
	if len(got) != len(want) {
		t.Fatalf("deleted set size = %d %v, want %d %v", len(got), got, len(want), want)
	}
	for _, id := range want {
		if !got[id] {
			t.Fatalf("expected id %d to be deleted; got %v", id, got)
		}
	}
}

// TestSelectGFSToDelete_DailyTier 验证按天分层：每天仅保留最新一份，且只保留最近 N 天。
func TestSelectGFSToDelete_DailyTier(t *testing.T) {
	records := []model.BackupRecord{
		gfsRecord(5, gfsDay(2026, 3, 7, 12), false), // 今天，最新 → 保留
		gfsRecord(4, gfsDay(2026, 3, 7, 6), false),  // 今天，较早 → 删除（非当天代表）
		gfsRecord(3, gfsDay(2026, 3, 6, 12), false), // 昨天 → 保留
		gfsRecord(2, gfsDay(2026, 3, 5, 12), false), // 前天 → 超出 daily=2 → 删除
		gfsRecord(1, gfsDay(2026, 3, 4, 12), false), // 更早 → 删除
	}
	del := selectGFSToDelete(records, 2, 0, 0, 0)
	assertDeleted(t, del, 4, 2, 1)
}

// TestSelectGFSToDelete_TierUnion 验证多层级取并集：月度层级保留日度层级会删除的旧备份。
func TestSelectGFSToDelete_TierUnion(t *testing.T) {
	records := []model.BackupRecord{
		gfsRecord(3, gfsDay(2026, 3, 7, 12), false),  // 3 月（最新）
		gfsRecord(2, gfsDay(2026, 2, 15, 12), false), // 2 月
		gfsRecord(1, gfsDay(2026, 1, 15, 12), false), // 1 月
	}
	// daily=1 只留 ID3；monthly=2 留最近两个月（3 月=ID3、2 月=ID2）。并集={3,2}，删除 ID1。
	del := selectGFSToDelete(records, 1, 0, 2, 0)
	assertDeleted(t, del, 1)
}

// TestSelectGFSToDelete_SkipsLocked 验证锁定记录即使超出所有层级也永不删除。
func TestSelectGFSToDelete_SkipsLocked(t *testing.T) {
	records := []model.BackupRecord{
		gfsRecord(3, gfsDay(2026, 3, 7, 12), false),
		gfsRecord(2, gfsDay(2026, 3, 6, 12), false),
		gfsRecord(1, gfsDay(2020, 1, 1, 12), true), // 远超 daily=1 但已锁定 → 不删
	}
	del := selectGFSToDelete(records, 1, 0, 0, 0)
	assertDeleted(t, del, 2) // 仅 ID2 被删；ID1 锁定豁免，ID3 为当日代表
}

func TestGFSEnabled(t *testing.T) {
	if gfsEnabled(&model.BackupTask{}) {
		t.Fatal("empty GFS config should be disabled")
	}
	if !gfsEnabled(&model.BackupTask{KeepWeekly: 4}) {
		t.Fatal("KeepWeekly>0 should enable GFS")
	}
}

// TestSelectRecordsToDelete_SkipsLocked 验证保留锁定（法律保留）的记录永不被选中删除，
// 即使它既超过保留期、又超过 maxBackups 名额。
func TestSelectRecordsToDelete_SkipsLocked(t *testing.T) {
	now := time.Date(2026, 3, 7, 16, 0, 0, 0, time.UTC)
	completedNew := now.Add(-24 * time.Hour)
	completedOld := now.Add(-15 * 24 * time.Hour)
	records := []model.BackupRecord{
		{ID: 3, CompletedAt: &completedNew},
		{ID: 2, CompletedAt: &completedNew},
		{ID: 1, CompletedAt: &completedOld, Locked: true}, // 超期但锁定 → 不应删除
	}
	selected := selectRecordsToDelete(records, 7, 2, now)
	for _, r := range selected {
		if r.ID == 1 {
			t.Fatalf("locked record #1 must never be selected for deletion: %#v", selected)
		}
	}
	// 锁定记录不占 maxBackups 名额：未锁定仅 2 条，maxBackups=2 → 无超额删除，
	// 且无未锁定记录超期 → 选中集为空。
	if len(selected) != 0 {
		t.Fatalf("expected no deletions (locked excluded), got %#v", selected)
	}
}

func TestCleanupDeletesExpiredRecords(t *testing.T) {
	now := time.Date(2026, 3, 7, 16, 0, 0, 0, time.UTC)
	completedNew := now.Add(-24 * time.Hour)
	completedOld := now.Add(-15 * 24 * time.Hour)
	repo := &fakeRecordRepository{records: []model.BackupRecord{
		{ID: 3, TaskID: 1, StoragePath: "records/3", CompletedAt: &completedNew},
		{ID: 2, TaskID: 1, StoragePath: "records/2", CompletedAt: &completedNew},
		{ID: 1, TaskID: 1, StoragePath: "records/1", CompletedAt: &completedOld},
	}}
	provider := &fakeProvider{}
	service := NewService(repo)
	service.now = func() time.Time { return now }
	result, err := service.Cleanup(context.Background(), &model.BackupTask{ID: 1, RetentionDays: 7, MaxBackups: 2}, provider)
	if err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}
	if result.DeletedRecords != 1 || result.DeletedObjects != 1 {
		t.Fatalf("unexpected cleanup result: %#v", result)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != 1 {
		t.Fatalf("unexpected deleted records: %#v", repo.deleted)
	}
	if len(provider.deleted) != 1 || provider.deleted[0] != "records/1" {
		t.Fatalf("unexpected deleted objects: %#v", provider.deleted)
	}
}
