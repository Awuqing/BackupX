package metrics

import (
	"context"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// SampleSource 抽象 Collector 需要的仓储访问，便于单测替换。
type SampleSource interface {
	ListStorageTargets(ctx context.Context) ([]model.StorageTarget, error)
	StorageUsage(ctx context.Context) ([]repository.BackupStorageUsageItem, error)
	ListNodes(ctx context.Context) ([]model.Node, error)
	CountSLABreach(ctx context.Context) (int, error)
}

// repoSource 把 repository 适配到 SampleSource。
type repoSource struct {
	targets repository.StorageTargetRepository
	records repository.BackupRecordRepository
	nodes   repository.NodeRepository
	tasks   repository.BackupTaskRepository
	now     func() time.Time
}

// NewRepoSource 用仓储实例构造 SampleSource。
func NewRepoSource(
	targets repository.StorageTargetRepository,
	records repository.BackupRecordRepository,
	nodes repository.NodeRepository,
	tasks repository.BackupTaskRepository,
) SampleSource {
	return &repoSource{
		targets: targets,
		records: records,
		nodes:   nodes,
		tasks:   tasks,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (s *repoSource) ListStorageTargets(ctx context.Context) ([]model.StorageTarget, error) {
	return s.targets.List(ctx)
}

func (s *repoSource) StorageUsage(ctx context.Context) ([]repository.BackupStorageUsageItem, error) {
	return s.records.StorageUsage(ctx)
}

func (s *repoSource) ListNodes(ctx context.Context) ([]model.Node, error) {
	return s.nodes.List(ctx)
}

// CountSLABreach 统计当前违反 RPO 的任务：
//   - 任务启用且配置了 SLAHoursRPO > 0
//   - 最近一次成功备份距今超出 SLA 时间窗，或从未成功过
func (s *repoSource) CountSLABreach(ctx context.Context) (int, error) {
	tasks, err := s.tasks.List(ctx, repository.BackupTaskListOptions{})
	if err != nil {
		return 0, err
	}
	now := s.now()
	count := 0
	for i := range tasks {
		task := &tasks[i]
		if task.SLAHoursRPO <= 0 || !task.Enabled {
			continue
		}
		threshold := now.Add(-time.Duration(task.SLAHoursRPO) * time.Hour)
		if task.LastRunAt == nil || task.LastRunAt.Before(threshold) {
			count++
		}
	}
	return count, nil
}

// Collector 周期性采集 gauge 类指标（存储用量、节点在线、SLA 违约）。
// 用后台 goroutine 驱动，避免在 /metrics 请求路径做慢 IO。
type Collector struct {
	metrics  *Metrics
	source   SampleSource
	interval time.Duration
}

// NewCollector 创建周期采集器。interval=0 走默认 30s。
func NewCollector(m *Metrics, source SampleSource, interval time.Duration) *Collector {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Collector{metrics: m, source: source, interval: interval}
}

// Start 在后台运行采集循环；随 ctx 取消而终止。
// 启动时立即采一次，之后按 interval 轮询。
func (c *Collector) Start(ctx context.Context) {
	if c == nil || c.metrics == nil || c.source == nil {
		return
	}
	go func() {
		c.collect(ctx)
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.collect(ctx)
			}
		}
	}()
}

// collect 执行一次采样；单轮失败不影响下次。
func (c *Collector) collect(ctx context.Context) {
	// 存储用量：按 StorageTargetID 聚合 file_size，对应 target name/type
	if targets, err := c.source.ListStorageTargets(ctx); err == nil {
		nameByID := make(map[uint]string, len(targets))
		typeByID := make(map[uint]string, len(targets))
		for i := range targets {
			nameByID[targets[i].ID] = targets[i].Name
			typeByID[targets[i].ID] = targets[i].Type
		}
		if usage, uerr := c.source.StorageUsage(ctx); uerr == nil {
			c.metrics.ResetStorageUsed()
			for _, item := range usage {
				name := nameByID[item.StorageTargetID]
				if name == "" {
					continue
				}
				c.metrics.SetStorageUsed(name, typeByID[item.StorageTargetID], item.TotalSize)
			}
		}
	}
	// 节点在线状态：role 约定为 master / agent
	if nodes, err := c.source.ListNodes(ctx); err == nil {
		c.metrics.ResetNodeOnline()
		for i := range nodes {
			n := &nodes[i]
			role := "agent"
			if n.IsLocal {
				role = "master"
			}
			c.metrics.SetNodeOnline(n.Name, role, n.Status == model.NodeStatusOnline)
		}
	}
	if breach, err := c.source.CountSLABreach(ctx); err == nil {
		c.metrics.SetSLABreach(breach)
	}
}
