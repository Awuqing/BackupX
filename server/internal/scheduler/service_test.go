package scheduler

import (
	"backupx/server/internal/repository"
	servicepkg "backupx/server/internal/service"
	"context"
	"testing"
	"time"

	"backupx/server/internal/model"
)

type fakeTaskRepository struct {
	items []model.BackupTask
}

func (r *fakeTaskRepository) List(context.Context, repository.BackupTaskListOptions) ([]model.BackupTask, error) {
	return nil, nil
}
func (r *fakeTaskRepository) FindByID(context.Context, uint) (*model.BackupTask, error) {
	return nil, nil
}
func (r *fakeTaskRepository) FindByName(context.Context, string) (*model.BackupTask, error) {
	return nil, nil
}
func (r *fakeTaskRepository) ListSchedulable(context.Context) ([]model.BackupTask, error) {
	return r.items, nil
}
func (r *fakeTaskRepository) ListVerifySchedulable(context.Context) ([]model.BackupTask, error) {
	return nil, nil
}
func (r *fakeTaskRepository) DistinctTags(context.Context) ([]string, error) {
	return nil, nil
}
func (r *fakeTaskRepository) Count(context.Context) (int64, error)        { return 0, nil }
func (r *fakeTaskRepository) CountEnabled(context.Context) (int64, error) { return 0, nil }
func (r *fakeTaskRepository) CountByStorageTargetID(context.Context, uint) (int64, error) {
	return 0, nil
}
func (r *fakeTaskRepository) CountByNodeID(context.Context, uint) (int64, error) {
	return 0, nil
}
func (r *fakeTaskRepository) ListByNodeID(context.Context, uint) ([]model.BackupTask, error) {
	return nil, nil
}
func (r *fakeTaskRepository) Create(context.Context, *model.BackupTask) error { return nil }
func (r *fakeTaskRepository) Update(context.Context, *model.BackupTask) error { return nil }
func (r *fakeTaskRepository) Delete(context.Context, uint) error              { return nil }

type fakeRunner struct{ taskIDs []uint }

func (r *fakeRunner) RunTaskByID(_ context.Context, id uint) (*servicepkg.BackupRecordDetail, error) {
	r.taskIDs = append(r.taskIDs, id)
	return nil, nil
}

func TestServiceSyncTaskAndTrigger(t *testing.T) {
	repo := &fakeTaskRepository{}
	runner := &fakeRunner{}
	service := NewService(repo, runner, nil)
	if err := service.SyncTask(context.Background(), &model.BackupTask{ID: 1, Enabled: true, CronExpr: "*/1 * * * * *"}); err != nil {
		t.Fatalf("SyncTask returned error: %v", err)
	}
	service.cron.Start()
	defer service.cron.Stop()
	time.Sleep(1100 * time.Millisecond)
	if len(runner.taskIDs) == 0 {
		t.Fatalf("expected scheduled runner to be triggered")
	}
}

func TestServiceSchedulesTasksInLocalTimezone(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("LoadLocation returned error: %v", err)
	}
	originalLocal := time.Local
	time.Local = location
	t.Cleanup(func() {
		time.Local = originalLocal
	})

	service := NewService(&fakeTaskRepository{}, &fakeRunner{}, nil)
	if got := service.cron.Location(); got != location {
		t.Fatalf("cron location = %v, want %v", got, location)
	}

	task := &model.BackupTask{ID: 1, Enabled: true, CronExpr: "0 5 * * *"}
	if err := service.SyncTask(context.Background(), task); err != nil {
		t.Fatalf("SyncTask returned error: %v", err)
	}
	entryID, ok := service.entries[task.ID]
	if !ok {
		t.Fatalf("expected cron entry for task %d", task.ID)
	}

	entry := service.cron.Entry(entryID)
	now := time.Date(2026, 4, 30, 4, 0, 0, 0, location)
	got := entry.Schedule.Next(now)
	want := time.Date(2026, 4, 30, 5, 0, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("next run = %s, want %s", got, want)
	}
}
