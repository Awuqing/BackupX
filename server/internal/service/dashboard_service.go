package service

import (
	"context"
	"time"

	"backupx/server/internal/apperror"
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
	tasks   repository.BackupTaskRepository
	records repository.BackupRecordRepository
	targets repository.StorageTargetRepository
}

func NewDashboardService(tasks repository.BackupTaskRepository, records repository.BackupRecordRepository, targets repository.StorageTargetRepository) *DashboardService {
	return &DashboardService{tasks: tasks, records: records, targets: targets}
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
