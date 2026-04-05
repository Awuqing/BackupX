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
