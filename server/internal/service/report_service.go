package service

import (
	"context"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// ReportService 生成企业合规报表。区别于 Dashboard 的实时聚合视图，
// 本服务产出「按任务、可导出、可归档」的时间点合规证据（供 SOC2 / ISO27001 等审计）。
type ReportService struct {
	tasks   repository.BackupTaskRepository
	records repository.BackupRecordRepository
}

func NewReportService(tasks repository.BackupTaskRepository, records repository.BackupRecordRepository) *ReportService {
	return &ReportService{tasks: tasks, records: records}
}

// ComplianceTaskRow 单个备份任务的合规明细行。
type ComplianceTaskRow struct {
	TaskID         uint       `json:"taskId"`
	TaskName       string     `json:"taskName"`
	Type           string     `json:"type"`
	Enabled        bool       `json:"enabled"`
	NodeName       string     `json:"nodeName"`
	CronExpr       string     `json:"cronExpr"`
	Encrypted      bool       `json:"encrypted"`
	RetentionDays  int        `json:"retentionDays"`
	SLAHoursRPO    int        `json:"slaHoursRpo"`
	TotalRuns      int        `json:"totalRuns"`
	Successes      int        `json:"successes"`
	Failures       int        `json:"failures"`
	SuccessRate    float64    `json:"successRate"`
	LastStatus     string     `json:"lastStatus"`
	LastRunAt      *time.Time `json:"lastRunAt,omitempty"`
	LastSuccessAt  *time.Time `json:"lastSuccessAt,omitempty"`
	ProtectedBytes int64      `json:"protectedBytes"`
	Compliant      bool       `json:"compliant"`
	Risk           string     `json:"risk"` // ok | at_risk | not_applicable
}

// ComplianceSummary 报表汇总。
type ComplianceSummary struct {
	TotalTasks         int     `json:"totalTasks"`
	EnabledTasks       int     `json:"enabledTasks"`
	CompliantTasks     int     `json:"compliantTasks"`
	AtRiskTasks        int     `json:"atRiskTasks"`
	EncryptedTasks     int     `json:"encryptedTasks"`
	OverallSuccessRate float64 `json:"overallSuccessRate"`
	TotalProtectedB    int64   `json:"totalProtectedBytes"`
}

// ComplianceReport 完整合规报表。
type ComplianceReport struct {
	GeneratedAt time.Time           `json:"generatedAt"`
	RangeDays   int                 `json:"rangeDays"`
	Summary     ComplianceSummary   `json:"summary"`
	Tasks       []ComplianceTaskRow `json:"tasks"`
}

const (
	reportMinDays = 1
	reportMaxDays = 365
)

// ComplianceReport 生成最近 days 天的合规报表。
func (s *ReportService) ComplianceReport(ctx context.Context, days int) (*ComplianceReport, error) {
	if days < reportMinDays || days > reportMaxDays {
		return nil, apperror.BadRequest("REPORT_RANGE_INVALID",
			"统计天数需在 1-365 之间", nil)
	}
	tasks, err := s.tasks.List(ctx, repository.BackupTaskListOptions{})
	if err != nil {
		return nil, apperror.Internal("REPORT_TASKS_FAILED", "无法获取任务列表", err)
	}
	now := time.Now().UTC()
	since := now.AddDate(0, 0, -days)

	report := &ComplianceReport{GeneratedAt: now, RangeDays: days, Tasks: make([]ComplianceTaskRow, 0, len(tasks))}
	var totalRuns, totalSuccess int
	for i := range tasks {
		task := tasks[i]
		records, err := s.records.ListByTask(ctx, task.ID)
		if err != nil {
			return nil, apperror.Internal("REPORT_RECORDS_FAILED", "无法获取任务备份记录", err)
		}
		row := s.buildTaskRow(&task, records, now, since)
		report.Tasks = append(report.Tasks, row)

		report.Summary.TotalTasks++
		if task.Enabled {
			report.Summary.EnabledTasks++
		}
		if task.Encrypt {
			report.Summary.EncryptedTasks++
		}
		switch row.Risk {
		case "ok":
			report.Summary.CompliantTasks++
		case "at_risk":
			report.Summary.AtRiskTasks++
		}
		report.Summary.TotalProtectedB += row.ProtectedBytes
		totalRuns += row.TotalRuns
		totalSuccess += row.Successes
	}
	if totalRuns > 0 {
		report.Summary.OverallSuccessRate = roundRate(float64(totalSuccess) / float64(totalRuns))
	}
	return report, nil
}

func (s *ReportService) buildTaskRow(task *model.BackupTask, records []model.BackupRecord, now, since time.Time) ComplianceTaskRow {
	row := ComplianceTaskRow{
		TaskID:        task.ID,
		TaskName:      task.Name,
		Type:          task.Type,
		Enabled:       task.Enabled,
		NodeName:      task.Node.Name,
		CronExpr:      task.CronExpr,
		Encrypted:     task.Encrypt,
		RetentionDays: task.RetentionDays,
		SLAHoursRPO:   task.SLAHoursRPO,
		LastStatus:    "none",
	}
	var lastRun, lastSuccess *model.BackupRecord
	for i := range records {
		rec := &records[i]
		// 统计窗口内的运行情况（按 StartedAt 落在 [since, now]）。
		if !rec.StartedAt.Before(since) {
			row.TotalRuns++
			switch rec.Status {
			case model.BackupRecordStatusSuccess:
				row.Successes++
			case model.BackupRecordStatusFailed:
				row.Failures++
			}
		}
		// 最近一次运行 / 最近一次成功（不限窗口，反映当前保护态）。
		if lastRun == nil || rec.StartedAt.After(lastRun.StartedAt) {
			lastRun = rec
		}
		if rec.Status == model.BackupRecordStatusSuccess {
			if lastSuccess == nil || rec.StartedAt.After(lastSuccess.StartedAt) {
				lastSuccess = rec
			}
		}
	}
	if row.TotalRuns > 0 {
		row.SuccessRate = roundRate(float64(row.Successes) / float64(row.TotalRuns))
	}
	if lastRun != nil {
		row.LastStatus = lastRun.Status
		started := lastRun.StartedAt
		row.LastRunAt = &started
	}
	if lastSuccess != nil {
		when := lastSuccess.StartedAt
		if lastSuccess.CompletedAt != nil {
			when = *lastSuccess.CompletedAt
		}
		row.LastSuccessAt = &when
		row.ProtectedBytes = lastSuccess.FileSize
	}
	row.Compliant, row.Risk = evaluateCompliance(task, lastSuccess, now)
	return row
}

// evaluateCompliance 判定任务合规性：
//   - 禁用任务：not_applicable（不计入合规/风险）。
//   - 从未成功：at_risk。
//   - 配置了 SLA（RPO 小时）：最近成功在 RPO 内为 ok，否则 at_risk。
//   - 未配置 SLA：只要存在成功备份即视为 ok。
func evaluateCompliance(task *model.BackupTask, lastSuccess *model.BackupRecord, now time.Time) (bool, string) {
	if !task.Enabled {
		return false, "not_applicable"
	}
	if lastSuccess == nil {
		return false, "at_risk"
	}
	if task.SLAHoursRPO > 0 {
		when := lastSuccess.StartedAt
		if lastSuccess.CompletedAt != nil {
			when = *lastSuccess.CompletedAt
		}
		if now.Sub(when).Hours() > float64(task.SLAHoursRPO) {
			return false, "at_risk"
		}
	}
	return true, "ok"
}

func roundRate(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}
