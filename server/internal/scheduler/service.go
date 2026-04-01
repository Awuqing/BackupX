package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	servicepkg "backupx/server/internal/service"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type TaskRunner interface {
	RunTaskByID(context.Context, uint) (*servicepkg.BackupRecordDetail, error)
}

// AuditRecorder 记录审计日志（可选依赖）
type AuditRecorder interface {
	Record(servicepkg.AuditEntry)
}

type Service struct {
	mu      sync.Mutex
	cron    *cron.Cron
	tasks   repository.BackupTaskRepository
	runner  TaskRunner
	logger  *zap.Logger
	audit   AuditRecorder
	entries map[uint]cron.EntryID
}

func NewService(tasks repository.BackupTaskRepository, runner TaskRunner, logger *zap.Logger) *Service {
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	return &Service{cron: cron.New(cron.WithParser(parser), cron.WithLocation(time.UTC)), tasks: tasks, runner: runner, logger: logger, entries: make(map[uint]cron.EntryID)}
}

func (s *Service) SetAuditRecorder(audit AuditRecorder) { s.audit = audit }

func (s *Service) Start(ctx context.Context) error {
	if err := s.Reload(ctx); err != nil {
		return err
	}
	s.cron.Start()
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	stopCtx := s.cron.Stop()
	select {
	case <-stopCtx.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) Reload(ctx context.Context) error {
	items, err := s.tasks.ListSchedulable(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for taskID, entryID := range s.entries {
		s.cron.Remove(entryID)
		delete(s.entries, taskID)
	}
	for _, item := range items {
		item := item
		if err := s.syncTaskLocked(&item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SyncTask(_ context.Context, task *model.BackupTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncTaskLocked(task)
}

func (s *Service) RemoveTask(_ context.Context, taskID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.entries[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, taskID)
	}
	return nil
}

func (s *Service) syncTaskLocked(task *model.BackupTask) error {
	if task == nil {
		return fmt.Errorf("task is required")
	}
	if entryID, ok := s.entries[task.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, task.ID)
	}
	if !task.Enabled || task.CronExpr == "" {
		return nil
	}
	taskID := task.ID
	taskName := task.Name
	entryID, err := s.cron.AddFunc(task.CronExpr, func() {
		// 自动调度任务记录审计日志
		if s.audit != nil {
			s.audit.Record(servicepkg.AuditEntry{
				Username: "system", Category: "backup_task", Action: "scheduled_run",
				TargetType: "backup_task", TargetID: fmt.Sprintf("%d", taskID),
				TargetName: taskName, Detail: fmt.Sprintf("定时调度触发备份任务: %s (cron: %s)", taskName, task.CronExpr),
			})
		}
		if _, runErr := s.runner.RunTaskByID(context.Background(), taskID); runErr != nil && s.logger != nil {
			s.logger.Warn("scheduled backup run failed", zap.Uint("task_id", taskID), zap.Error(runErr))
		}
	})
	if err != nil {
		return err
	}
	s.entries[task.ID] = entryID
	return nil
}
