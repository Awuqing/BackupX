package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backupx/server/internal/backup"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	servicepkg "backupx/server/internal/service"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type TaskRunner interface {
	RunTaskByID(context.Context, uint) (*servicepkg.BackupRecordDetail, error)
}

// VerifyRunner 供调度器触发验证演练。
// 使用最新成功备份作为源；taskID 对应的任务须配置 VerifyEnabled=true。
type VerifyRunner interface {
	StartByTask(ctx context.Context, taskID uint, mode, triggeredBy string) (*servicepkg.VerificationRecordDetail, error)
}

// AuditRecorder 记录审计日志（可选依赖）
type AuditRecorder interface {
	Record(servicepkg.AuditEntry)
}

type Service struct {
	mu            sync.Mutex
	cron          *cron.Cron
	tasks         repository.BackupTaskRepository
	nodes         repository.NodeRepository
	runner        TaskRunner
	verifyRunner  VerifyRunner
	logger        *zap.Logger
	audit         AuditRecorder
	entries       map[uint]cron.EntryID // 备份 cron 条目
	verifyEntries map[uint]cron.EntryID // 验证 cron 条目
}

func NewService(tasks repository.BackupTaskRepository, runner TaskRunner, logger *zap.Logger) *Service {
	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	return &Service{
		cron:          cron.New(cron.WithParser(parser), cron.WithLocation(time.UTC)),
		tasks:         tasks,
		runner:        runner,
		logger:        logger,
		entries:       make(map[uint]cron.EntryID),
		verifyEntries: make(map[uint]cron.EntryID),
	}
}

// SetVerifyRunner 注入验证调度器。可选注入：未注入时不处理验证 cron。
func (s *Service) SetVerifyRunner(runner VerifyRunner) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verifyRunner = runner
}

func (s *Service) SetAuditRecorder(audit AuditRecorder) { s.audit = audit }

// SetNodeRepository 注入节点仓储用于调度前的健康检查。
// 可选注入：未注入时调度器无条件触发任务（单节点场景）。
func (s *Service) SetNodeRepository(nodes repository.NodeRepository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes = nodes
}

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
	// 验证调度单独扫描（启用验证的任务可能未启用备份 cron，反之亦然）
	verifyItems, err := s.tasks.ListVerifySchedulable(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for taskID, entryID := range s.entries {
		s.cron.Remove(entryID)
		delete(s.entries, taskID)
	}
	for taskID, entryID := range s.verifyEntries {
		s.cron.Remove(entryID)
		delete(s.verifyEntries, taskID)
	}
	for _, item := range items {
		item := item
		if err := s.syncTaskLocked(&item); err != nil {
			return err
		}
	}
	for _, item := range verifyItems {
		item := item
		if err := s.syncVerifyTaskLocked(&item); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SyncTask(_ context.Context, task *model.BackupTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.syncTaskLocked(task); err != nil {
		return err
	}
	return s.syncVerifyTaskLocked(task)
}

func (s *Service) RemoveTask(_ context.Context, taskID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.entries[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, taskID)
	}
	if entryID, ok := s.verifyEntries[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.verifyEntries, taskID)
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
	taskNodeID := task.NodeID
	cronExpr := task.CronExpr
	maintenanceWindows := task.MaintenanceWindows
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		// 集群感知：若任务绑定了离线的远程节点，跳过本轮触发避免堆积 failed 记录
		if taskNodeID > 0 && s.nodes != nil {
			node, err := s.nodes.FindByID(context.Background(), taskNodeID)
			if err == nil && node != nil && !node.IsLocal && node.Status != model.NodeStatusOnline {
				if s.logger != nil {
					s.logger.Warn("skip scheduled run: target node offline",
						zap.Uint("task_id", taskID), zap.String("task_name", taskName),
						zap.Uint("node_id", taskNodeID), zap.String("node_name", node.Name))
				}
				if s.audit != nil {
					s.audit.Record(servicepkg.AuditEntry{
						Username: "system", Category: "backup_task", Action: "scheduled_skip",
						TargetType: "backup_task", TargetID: fmt.Sprintf("%d", taskID),
						TargetName: taskName,
						Detail:     fmt.Sprintf("跳过调度触发：节点 %s 离线 (task: %s, cron: %s)", node.Name, taskName, cronExpr),
					})
				}
				return
			}
		}
		// 维护窗口校验：非窗口时间跳过。Windows 为空则不限制。
		if maintenanceWindows != "" {
			windows := backup.ParseMaintenanceWindows(maintenanceWindows)
			if len(windows) > 0 && !backup.IsWithinWindow(time.Now(), windows) {
				if s.logger != nil {
					s.logger.Info("skip scheduled run: outside maintenance window",
						zap.Uint("task_id", taskID), zap.String("task_name", taskName),
						zap.String("windows", maintenanceWindows))
				}
				if s.audit != nil {
					s.audit.Record(servicepkg.AuditEntry{
						Username: "system", Category: "backup_task", Action: "scheduled_skip",
						TargetType: "backup_task", TargetID: fmt.Sprintf("%d", taskID),
						TargetName: taskName,
						Detail:     fmt.Sprintf("跳过调度触发：非维护窗口 (task: %s, windows: %s)", taskName, maintenanceWindows),
					})
				}
				return
			}
		}
		// 自动调度任务记录审计日志
		if s.audit != nil {
			s.audit.Record(servicepkg.AuditEntry{
				Username: "system", Category: "backup_task", Action: "scheduled_run",
				TargetType: "backup_task", TargetID: fmt.Sprintf("%d", taskID),
				TargetName: taskName, Detail: fmt.Sprintf("定时调度触发备份任务: %s (cron: %s)", taskName, cronExpr),
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

// syncVerifyTaskLocked 同步任务的验证演练 cron 条目。
// 调度时间到 → 拉取最新成功备份 → 触发 Verify 快速校验。
// 若未注入 verifyRunner，直接返回（单节点+无验证场景）。
func (s *Service) syncVerifyTaskLocked(task *model.BackupTask) error {
	if task == nil {
		return fmt.Errorf("task is required")
	}
	if entryID, ok := s.verifyEntries[task.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.verifyEntries, task.ID)
	}
	if s.verifyRunner == nil {
		return nil
	}
	if !task.Enabled || !task.VerifyEnabled || task.VerifyCronExpr == "" {
		return nil
	}
	taskID := task.ID
	taskName := task.Name
	mode := task.VerifyMode
	verifyCron := task.VerifyCronExpr
	entryID, err := s.cron.AddFunc(verifyCron, func() {
		if s.audit != nil {
			s.audit.Record(servicepkg.AuditEntry{
				Username: "system", Category: "backup_verify", Action: "scheduled_run",
				TargetType: "backup_task", TargetID: fmt.Sprintf("%d", taskID),
				TargetName: taskName, Detail: fmt.Sprintf("定时验证演练: %s (cron: %s, mode: %s)", taskName, verifyCron, mode),
			})
		}
		if _, runErr := s.verifyRunner.StartByTask(context.Background(), taskID, mode, "system"); runErr != nil && s.logger != nil {
			s.logger.Warn("scheduled verify run failed", zap.Uint("task_id", taskID), zap.Error(runErr))
		}
	})
	if err != nil {
		return err
	}
	s.verifyEntries[task.ID] = entryID
	return nil
}
