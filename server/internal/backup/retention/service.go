package retention

import (
	"context"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
)

// collectDirPrefixes 从待删除的记录中提取唯一的父目录前缀。
func collectDirPrefixes(records []model.BackupRecord) []string {
	seen := make(map[string]struct{})
	var prefixes []string
	for _, record := range records {
		path := strings.TrimSpace(record.StoragePath)
		if path == "" {
			continue
		}
		idx := strings.LastIndex(path, "/")
		if idx <= 0 {
			continue
		}
		dir := path[:idx]
		if _, ok := seen[dir]; !ok {
			seen[dir] = struct{}{}
			prefixes = append(prefixes, dir)
		}
	}
	return prefixes
}

type CleanupResult struct {
	DeletedRecords int
	DeletedObjects int
	Warnings       []string
}

type Service struct {
	records repository.BackupRecordRepository
	now     func() time.Time
}

func NewService(records repository.BackupRecordRepository) *Service {
	return &Service{records: records, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Cleanup(ctx context.Context, task *model.BackupTask, provider storage.StorageProvider) (*CleanupResult, error) {
	if task == nil {
		return nil, fmt.Errorf("backup task is required")
	}
	records, err := s.records.ListSuccessfulByTask(ctx, task.ID)
	if err != nil {
		return nil, fmt.Errorf("list successful records: %w", err)
	}
	candidates := selectRecordsToDelete(records, task.RetentionDays, task.MaxBackups, s.now())
	result := &CleanupResult{}
	for _, record := range candidates {
		if strings.TrimSpace(record.StoragePath) != "" {
			if provider == nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("record %d missing storage provider for cleanup", record.ID))
				continue
			}
			if err := provider.Delete(ctx, record.StoragePath); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("delete storage object %s failed: %v", record.StoragePath, err))
				continue
			}
			result.DeletedObjects++
		}
		if err := s.records.Delete(ctx, record.ID); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("delete backup record %d failed: %v", record.ID, err))
			continue
		}
		result.DeletedRecords++
	}

	// 清理空目录：收集被删除文件的父目录，尝试移除空目录
	if dirCleaner, ok := provider.(storage.StorageDirCleaner); ok && result.DeletedObjects > 0 {
		prefixes := collectDirPrefixes(candidates)
		for _, prefix := range prefixes {
			if err := dirCleaner.RemoveEmptyDirs(ctx, prefix); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("cleanup empty dirs for %s: %v", prefix, err))
			}
		}
	}

	return result, nil
}

func selectRecordsToDelete(records []model.BackupRecord, retentionDays int, maxBackups int, now time.Time) []model.BackupRecord {
	selected := make(map[uint]model.BackupRecord)
	if maxBackups > 0 && len(records) > maxBackups {
		for _, record := range records[maxBackups:] {
			selected[record.ID] = record
		}
	}
	if retentionDays > 0 {
		cutoff := now.AddDate(0, 0, -retentionDays)
		for _, record := range records {
			if record.CompletedAt != nil && record.CompletedAt.Before(cutoff) {
				selected[record.ID] = record
			}
		}
	}
	result := make([]model.BackupRecord, 0, len(selected))
	for _, record := range records {
		if selectedRecord, ok := selected[record.ID]; ok {
			result = append(result, selectedRecord)
		}
	}
	return result
}
