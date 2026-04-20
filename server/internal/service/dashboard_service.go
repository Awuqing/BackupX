package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

type DashboardStorageUsageItem struct {
	StorageTargetID uint   `json:"storageTargetId"`
	TargetName      string `json:"targetName"`
	TotalSize       int64  `json:"totalSize"`
}

type DashboardStats struct {
	TotalTasks       int64                       `json:"totalTasks"`
	EnabledTasks     int64                       `json:"enabledTasks"`
	TotalRecords     int64                       `json:"totalRecords"`
	SuccessRate      float64                     `json:"successRate"`
	TotalBackupBytes int64                       `json:"totalBackupBytes"`
	LastBackupAt     *time.Time                  `json:"lastBackupAt,omitempty"`
	RecentRecords    []BackupRecordSummary       `json:"recentRecords"`
	StorageUsage     []DashboardStorageUsageItem `json:"storageUsage"`
}

type DashboardService struct {
	tasks         repository.BackupTaskRepository
	records       repository.BackupRecordRepository
	targets       repository.StorageTargetRepository
	nodes         repository.NodeRepository
	masterVersion string
	// slaMonitor 内部跟踪已告警的违约任务，避免每次扫描重复派发事件
	slaNotified map[uint]time.Time
	slaMu       sync.Mutex
}

func NewDashboardService(tasks repository.BackupTaskRepository, records repository.BackupRecordRepository, targets repository.StorageTargetRepository) *DashboardService {
	return &DashboardService{tasks: tasks, records: records, targets: targets, slaNotified: map[uint]time.Time{}}
}

// SetClusterDependencies 注入节点仓储与 Master 版本，启用集群概览。
func (s *DashboardService) SetClusterDependencies(nodes repository.NodeRepository, masterVersion string) {
	s.nodes = nodes
	s.masterVersion = masterVersion
}

func (s *DashboardService) Stats(ctx context.Context) (*DashboardStats, error) {
	totalTasks, err := s.tasks.Count(ctx)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_STATS_FAILED", "无法统计备份任务数量", err)
	}
	enabledTasks, err := s.tasks.CountEnabled(ctx)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_STATS_FAILED", "无法统计启用任务数量", err)
	}
	totalRecords, err := s.records.Count(ctx)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_STATS_FAILED", "无法统计备份记录数量", err)
	}
	since := time.Now().UTC().AddDate(0, 0, -30)
	recentRecordsCount, err := s.records.CountSince(ctx, since)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_STATS_FAILED", "无法统计最近记录数量", err)
	}
	successRecordsCount, err := s.records.CountSuccessSince(ctx, since)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_STATS_FAILED", "无法统计最近成功记录数量", err)
	}
	totalBackupBytes, err := s.records.SumFileSize(ctx)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_STATS_FAILED", "无法统计备份总量", err)
	}
	recentRecords, err := s.records.ListRecent(ctx, 10)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_STATS_FAILED", "无法获取最近备份记录", err)
	}
	targetList, err := s.targets.List(ctx)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_STATS_FAILED", "无法获取存储目标信息", err)
	}
	targetNames := make(map[uint]string, len(targetList))
	for _, item := range targetList {
		targetNames[item.ID] = item.Name
	}
	usageItems, err := s.records.StorageUsage(ctx)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_STATS_FAILED", "无法统计存储使用量", err)
	}
	storageUsage := make([]DashboardStorageUsageItem, 0, len(usageItems))
	for _, item := range usageItems {
		storageUsage = append(storageUsage, DashboardStorageUsageItem{StorageTargetID: item.StorageTargetID, TargetName: targetNames[item.StorageTargetID], TotalSize: item.TotalSize})
	}
	result := &DashboardStats{TotalTasks: totalTasks, EnabledTasks: enabledTasks, TotalRecords: totalRecords, TotalBackupBytes: totalBackupBytes, RecentRecords: make([]BackupRecordSummary, 0, len(recentRecords)), StorageUsage: storageUsage}
	if recentRecordsCount > 0 {
		result.SuccessRate = float64(successRecordsCount) / float64(recentRecordsCount)
	}
	if len(recentRecords) > 0 {
		result.LastBackupAt = &recentRecords[0].StartedAt
	}
	for _, item := range recentRecords {
		result.RecentRecords = append(result.RecentRecords, toBackupRecordSummary(&item))
	}
	return result, nil
}

func (s *DashboardService) Timeline(ctx context.Context, days int) ([]repository.BackupTimelinePoint, error) {
	if days <= 0 {
		days = 30
	}
	items, err := s.records.TimelineSince(ctx, time.Now().UTC().AddDate(0, 0, -days))
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_TIMELINE_FAILED", "无法获取备份时间线", err)
	}
	if items == nil {
		items = []repository.BackupTimelinePoint{}
	}
	return items, nil
}

// SLAViolation 任务 SLA 违约详情。
// 判定规则：任务设置了 SLAHoursRPO > 0，且距最近一次 success 备份的时间 > SLAHoursRPO。
// 从未成功过的任务（LastSuccessAt = nil）若启用也视为违约（from createdAt 起算）。
type SLAViolation struct {
	TaskID                 uint       `json:"taskId"`
	TaskName               string     `json:"taskName"`
	NodeID                 uint       `json:"nodeId"`
	NodeName               string     `json:"nodeName,omitempty"`
	SLAHoursRPO            int        `json:"slaHoursRpo"`
	LastSuccessAt          *time.Time `json:"lastSuccessAt,omitempty"`
	HoursSinceLastSuccess  float64    `json:"hoursSinceLastSuccess"`
	NeverSucceeded         bool       `json:"neverSucceeded"`
}

// SLAComplianceReport Dashboard 的 SLA 合规概览。
type SLAComplianceReport struct {
	TotalTasksWithSLA int            `json:"totalTasksWithSla"`
	Compliant         int            `json:"compliant"`
	Violated          int            `json:"violated"`
	CoverageRate      float64        `json:"coverageRate"`
	Violations        []SLAViolation `json:"violations"`
}

// SLACompliance 计算所有启用任务的 SLA 合规情况。
// 只考虑 Enabled=true 且 SLAHoursRPO>0 的任务。
func (s *DashboardService) SLACompliance(ctx context.Context) (*SLAComplianceReport, error) {
	items, err := s.tasks.List(ctx, repository.BackupTaskListOptions{})
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_SLA_FAILED", "无法获取任务列表", err)
	}
	now := time.Now().UTC()
	report := &SLAComplianceReport{Violations: []SLAViolation{}}
	for i := range items {
		task := items[i]
		if !task.Enabled || task.SLAHoursRPO <= 0 {
			continue
		}
		report.TotalTasksWithSLA++
		// 查最近的成功记录作为 lastSuccessAt
		successes, err := s.records.ListSuccessfulByTask(ctx, task.ID)
		if err != nil {
			return nil, apperror.Internal("DASHBOARD_SLA_FAILED", "无法获取任务成功记录", err)
		}
		var lastSuccessAt *time.Time
		if len(successes) > 0 && successes[0].CompletedAt != nil {
			lastSuccessAt = successes[0].CompletedAt
		}
		hoursSince := 0.0
		neverSucceeded := lastSuccessAt == nil
		if neverSucceeded {
			hoursSince = now.Sub(task.CreatedAt).Hours()
		} else {
			hoursSince = now.Sub(*lastSuccessAt).Hours()
		}
		if hoursSince > float64(task.SLAHoursRPO) {
			report.Violated++
			report.Violations = append(report.Violations, SLAViolation{
				TaskID:                task.ID,
				TaskName:              task.Name,
				NodeID:                task.NodeID,
				NodeName:              task.Node.Name,
				SLAHoursRPO:           task.SLAHoursRPO,
				LastSuccessAt:         lastSuccessAt,
				HoursSinceLastSuccess: roundHours(hoursSince),
				NeverSucceeded:        neverSucceeded,
			})
		} else {
			report.Compliant++
		}
	}
	if report.TotalTasksWithSLA > 0 {
		report.CoverageRate = float64(report.Compliant) / float64(report.TotalTasksWithSLA)
	}
	return report, nil
}

func roundHours(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

// ClusterNodeSummary 集群节点简报（Dashboard 用）。
type ClusterNodeSummary struct {
	ID             uint      `json:"id"`
	Name           string    `json:"name"`
	Hostname       string    `json:"hostname"`
	Status         string    `json:"status"`
	IsLocal        bool      `json:"isLocal"`
	AgentVersion   string    `json:"agentVersion"`
	VersionStatus  string    `json:"versionStatus"` // current | outdated | unknown
	LastSeen       time.Time `json:"lastSeen"`
	TaskCount      int64     `json:"taskCount"`
}

// ClusterOverview Dashboard 集群概览卡片。
type ClusterOverview struct {
	MasterVersion  string               `json:"masterVersion"`
	TotalNodes     int                  `json:"totalNodes"`
	OnlineNodes    int                  `json:"onlineNodes"`
	OfflineNodes   int                  `json:"offlineNodes"`
	OutdatedAgents int                  `json:"outdatedAgents"`
	Nodes          []ClusterNodeSummary `json:"nodes"`
}

// ClusterOverview 返回集群节点状态概览，未启用集群依赖时返回空对象。
func (s *DashboardService) ClusterOverview(ctx context.Context) (*ClusterOverview, error) {
	if s.nodes == nil {
		return &ClusterOverview{MasterVersion: s.masterVersion, Nodes: []ClusterNodeSummary{}}, nil
	}
	nodes, err := s.nodes.List(ctx)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_CLUSTER_FAILED", "无法获取节点列表", err)
	}
	out := &ClusterOverview{
		MasterVersion: s.masterVersion,
		TotalNodes:    len(nodes),
		Nodes:         make([]ClusterNodeSummary, 0, len(nodes)),
	}
	for i := range nodes {
		node := nodes[i]
		var taskCount int64
		if s.tasks != nil {
			if c, err := s.tasks.CountByNodeID(ctx, node.ID); err == nil {
				taskCount = c
			}
		}
		versionStatus := resolveVersionStatus(node, s.masterVersion)
		summary := ClusterNodeSummary{
			ID:            node.ID,
			Name:          node.Name,
			Hostname:      node.Hostname,
			Status:        node.Status,
			IsLocal:       node.IsLocal,
			AgentVersion:  node.AgentVer,
			VersionStatus: versionStatus,
			LastSeen:      node.LastSeen,
			TaskCount:     taskCount,
		}
		out.Nodes = append(out.Nodes, summary)
		switch node.Status {
		case model.NodeStatusOnline:
			out.OnlineNodes++
		case model.NodeStatusOffline:
			out.OfflineNodes++
		}
		if versionStatus == "outdated" {
			out.OutdatedAgents++
		}
	}
	return out, nil
}

// BreakdownItem 单项分组统计。
type BreakdownItem struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Count     int64  `json:"count"`
	TotalSize int64  `json:"totalSize,omitempty"`
}

// BreakdownStats 多维分组统计。
type BreakdownStats struct {
	ByType    []BreakdownItem `json:"byType"`
	ByStatus  []BreakdownItem `json:"byStatus"`
	ByNode    []BreakdownItem `json:"byNode"`
	ByStorage []BreakdownItem `json:"byStorage"`
}

// Breakdown 返回多维分组统计。
// 仅统计最近 N 天的备份记录（默认 30 天），覆盖企业常见"近期分布"视角。
func (s *DashboardService) Breakdown(ctx context.Context, days int) (*BreakdownStats, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	// 按类型分组：来自 task 维度聚合
	tasks, err := s.tasks.List(ctx, repository.BackupTaskListOptions{})
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_BREAKDOWN_FAILED", "无法统计任务分组", err)
	}
	typeCounts := map[string]int64{}
	nodeCounts := map[uint]int64{}
	nodeNames := map[uint]string{0: "本机 Master"}
	for _, task := range tasks {
		typeCounts[task.Type]++
		nodeCounts[task.NodeID]++
		if task.Node.Name != "" {
			nodeNames[task.NodeID] = task.Node.Name
		}
	}
	result := &BreakdownStats{
		ByType:   makeBreakdown(typeCounts, typeLabel),
		ByNode:   makeBreakdownByUint(nodeCounts, nodeNames, "节点 #"),
		ByStatus: []BreakdownItem{},
		ByStorage: []BreakdownItem{},
	}
	// 按状态（最近 days 天记录）
	statusCounts, err := s.countRecordsByStatus(ctx, since)
	if err == nil {
		result.ByStatus = statusCounts
	}
	// 按存储目标（含字节数）
	if s.records != nil {
		storageItems, _ := s.records.StorageUsage(ctx)
		if s.targets != nil {
			targetNames := map[uint]string{}
			if targetList, err := s.targets.List(ctx); err == nil {
				for _, t := range targetList {
					targetNames[t.ID] = t.Name
				}
			}
			for _, item := range storageItems {
				name := targetNames[item.StorageTargetID]
				if name == "" {
					name = fmt.Sprintf("存储 #%d", item.StorageTargetID)
				}
				result.ByStorage = append(result.ByStorage, BreakdownItem{
					Key:       fmt.Sprintf("%d", item.StorageTargetID),
					Label:     name,
					TotalSize: item.TotalSize,
				})
			}
		}
	}
	return result, nil
}

// countRecordsByStatus 最近 since 起的记录按状态分组。
func (s *DashboardService) countRecordsByStatus(ctx context.Context, since time.Time) ([]BreakdownItem, error) {
	running, _ := s.records.List(ctx, repository.BackupRecordListOptions{Status: "running", DateFrom: &since})
	success, _ := s.records.List(ctx, repository.BackupRecordListOptions{Status: "success", DateFrom: &since})
	failed, _ := s.records.List(ctx, repository.BackupRecordListOptions{Status: "failed", DateFrom: &since})
	return []BreakdownItem{
		{Key: "success", Label: "成功", Count: int64(len(success))},
		{Key: "failed", Label: "失败", Count: int64(len(failed))},
		{Key: "running", Label: "执行中", Count: int64(len(running))},
	}, nil
}

// makeBreakdown 把 map[string]int64 转为排序好的 BreakdownItem 列表。
func makeBreakdown(counts map[string]int64, labelFn func(string) string) []BreakdownItem {
	items := make([]BreakdownItem, 0, len(counts))
	for k, v := range counts {
		label := k
		if labelFn != nil {
			label = labelFn(k)
		}
		items = append(items, BreakdownItem{Key: k, Label: label, Count: v})
	}
	// 按 Count 降序
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Count > items[i].Count {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	return items
}

func makeBreakdownByUint(counts map[uint]int64, names map[uint]string, fallback string) []BreakdownItem {
	items := make([]BreakdownItem, 0, len(counts))
	for k, v := range counts {
		label := names[k]
		if label == "" {
			label = fmt.Sprintf("%s%d", fallback, k)
		}
		items = append(items, BreakdownItem{Key: fmt.Sprintf("%d", k), Label: label, Count: v})
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Count > items[i].Count {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	return items
}

func typeLabel(key string) string {
	switch key {
	case "file":
		return "文件"
	case "mysql":
		return "MySQL"
	case "postgresql":
		return "PostgreSQL"
	case "sqlite":
		return "SQLite"
	case "saphana":
		return "SAP HANA"
	default:
		return key
	}
}

// NodePerformance 单节点近 N 天的执行指标。
// 用途：Dashboard 运维视角快速判断"哪个节点负载高 / 失败多 / 慢"。
type NodePerformance struct {
	NodeID          uint    `json:"nodeId"`
	NodeName        string  `json:"nodeName"`
	IsLocal         bool    `json:"isLocal"`
	TotalRuns       int     `json:"totalRuns"`
	SuccessRuns     int     `json:"successRuns"`
	FailedRuns      int     `json:"failedRuns"`
	SuccessRate     float64 `json:"successRate"`
	TotalBytes      int64   `json:"totalBytes"`
	AvgDurationSecs float64 `json:"avgDurationSecs"`
}

// NodePerformance 统计最近 days 天各节点的执行指标。
// 返回按成功率降序排列。未注入 nodeRepo 时返回空。
func (s *DashboardService) NodePerformance(ctx context.Context, days int) ([]NodePerformance, error) {
	if s.nodes == nil || s.records == nil {
		return []NodePerformance{}, nil
	}
	if days <= 0 {
		days = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	nodes, err := s.nodes.List(ctx)
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_NODE_PERF_FAILED", "无法获取节点列表", err)
	}
	// records 里没有直接的 node_id（通过 BackupTask.NodeID 关联）；
	// 先取近 N 天全部记录，按 record.NodeID 聚合（该字段已在第二轮加入）。
	items, err := s.records.List(ctx, repository.BackupRecordListOptions{DateFrom: &since})
	if err != nil {
		return nil, apperror.Internal("DASHBOARD_NODE_PERF_FAILED", "无法获取备份记录", err)
	}
	bucket := map[uint]*nodeAgg{}
	for i := range items {
		r := items[i]
		a, ok := bucket[r.NodeID]
		if !ok {
			a = &nodeAgg{}
			bucket[r.NodeID] = a
		}
		a.total++
		switch r.Status {
		case model.BackupRecordStatusSuccess:
			a.success++
			a.bytes += r.FileSize
			a.durSecs += int64(r.DurationSeconds)
		case model.BackupRecordStatusFailed:
			a.failed++
		}
	}
	out := make([]NodePerformance, 0, len(nodes)+1)
	// 确保"本机 Master"(id=0) 也被纳入，即便无记录
	seenLocal := false
	for _, n := range nodes {
		a := bucket[n.ID]
		if a == nil {
			a = &nodeAgg{}
		}
		perf := buildNodePerformance(n.ID, n.Name, n.IsLocal, a)
		out = append(out, perf)
		if n.ID == 0 || n.IsLocal {
			seenLocal = true
		}
	}
	// 若 bucket 里还有 id=0（未注册的 Master）或记录绑定的 node 已被删，追加"其他"
	if a, ok := bucket[0]; ok && !seenLocal {
		out = append(out, buildNodePerformance(0, "本机 Master", true, a))
	}
	// 按成功率降序，其次按 totalRuns 降序
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].SuccessRate > out[i].SuccessRate ||
				(out[j].SuccessRate == out[i].SuccessRate && out[j].TotalRuns > out[i].TotalRuns) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

// nodeAgg 按节点汇总的中间聚合结构（性能统计用）。
type nodeAgg struct {
	total, success, failed int
	bytes                  int64
	durSecs                int64
}

func buildNodePerformance(nodeID uint, nodeName string, isLocal bool, a *nodeAgg) NodePerformance {
	rate := 0.0
	if a.total > 0 {
		rate = float64(a.success) / float64(a.total)
	}
	avgDur := 0.0
	if a.success > 0 {
		avgDur = float64(a.durSecs) / float64(a.success)
	}
	return NodePerformance{
		NodeID:          nodeID,
		NodeName:        nodeName,
		IsLocal:         isLocal,
		TotalRuns:       a.total,
		SuccessRuns:     a.success,
		FailedRuns:      a.failed,
		SuccessRate:     rate,
		TotalBytes:      a.bytes,
		AvgDurationSecs: avgDur,
	}
}

// resolveVersionStatus 判断单个节点的版本健康度标签。
func resolveVersionStatus(node model.Node, masterVersion string) string {
	if node.IsLocal {
		return "current"
	}
	if node.AgentVer == "" {
		return "unknown"
	}
	if isClusterVersionOutdated(node.AgentVer, masterVersion) {
		return "outdated"
	}
	return "current"
}

// isClusterVersionOutdated 内部版本比较（与 cluster_version.go 语义一致）。
// 独立实现避免 service 包内跨文件耦合测试。
func isClusterVersionOutdated(agent, master string) bool {
	return isVersionOutdated(agent, master)
}

// StartSLAMonitor 后台定时扫描 SLA 违约并通过 event dispatcher 派发 sla_violation 事件。
// 防骚扰：同一任务在 resetInterval 内只派发一次（避免每分钟轰炸）。
//   - scanInterval：扫描频率（建议 15m）
//   - resetInterval：同任务再次告警的最短间隔（建议 6h）
//
// ctx 被取消时退出。dispatcher 为 nil 时退化为仅扫描不告警（保持兼容）。
func (s *DashboardService) StartSLAMonitor(ctx context.Context, dispatcher EventDispatcher, scanInterval, resetInterval time.Duration) {
	if scanInterval <= 0 {
		scanInterval = 15 * time.Minute
	}
	if resetInterval <= 0 {
		resetInterval = 6 * time.Hour
	}
	ticker := time.NewTicker(scanInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.scanAndDispatchSLA(ctx, dispatcher, resetInterval)
			}
		}
	}()
}

// scanAndDispatchSLA 执行一次 SLA 违约扫描并按需派发事件。
func (s *DashboardService) scanAndDispatchSLA(ctx context.Context, dispatcher EventDispatcher, resetInterval time.Duration) {
	report, err := s.SLACompliance(ctx)
	if err != nil || report == nil {
		return
	}
	now := time.Now().UTC()
	s.slaMu.Lock()
	defer s.slaMu.Unlock()
	// 保留当前仍然违约的任务，清理已恢复的记忆
	active := map[uint]time.Time{}
	violatingIDs := map[uint]bool{}
	for _, v := range report.Violations {
		violatingIDs[v.TaskID] = true
	}
	for taskID, when := range s.slaNotified {
		if violatingIDs[taskID] {
			active[taskID] = when
		}
	}
	s.slaNotified = active

	for _, v := range report.Violations {
		last, seen := s.slaNotified[v.TaskID]
		if seen && now.Sub(last) < resetInterval {
			continue
		}
		if dispatcher != nil {
			title := "BackupX SLA 违约"
			statusText := fmt.Sprintf("%.1f 小时", v.HoursSinceLastSuccess)
			if v.NeverSucceeded {
				statusText = "从未成功"
			}
			body := fmt.Sprintf("任务：%s\nRPO 目标：%d 小时\n距最近成功：%s", v.TaskName, v.SLAHoursRPO, statusText)
			fields := map[string]any{
				"taskId":                v.TaskID,
				"taskName":              v.TaskName,
				"nodeId":                v.NodeID,
				"nodeName":              v.NodeName,
				"slaHoursRpo":           v.SLAHoursRPO,
				"hoursSinceLastSuccess": v.HoursSinceLastSuccess,
				"neverSucceeded":        v.NeverSucceeded,
			}
			_ = dispatcher.DispatchEvent(ctx, model.NotificationEventSLAViolation, title, body, fields)
		}
		s.slaNotified[v.TaskID] = now
	}
}
