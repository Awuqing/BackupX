package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"backupx/server/internal/backup"
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

type cleanupObject struct {
	targetID uint
	path     string
}

type storedUploadResult struct {
	StorageTargetID uint   `json:"storageTargetId"`
	Status          string `json:"status"`
	StoragePath     string `json:"storagePath"`
}

type Service struct {
	records       repository.BackupRecordRepository
	repositoryKey []byte
	now           func() time.Time
}

func NewService(records repository.BackupRecordRepository, repositoryKey ...[]byte) *Service {
	var key []byte
	if len(repositoryKey) > 0 {
		key = append([]byte(nil), repositoryKey[0]...)
	}
	return &Service{records: records, repositoryKey: key, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) Cleanup(ctx context.Context, task *model.BackupTask, provider storage.StorageProvider) (*CleanupResult, error) {
	return s.cleanup(ctx, task, func(uint) (storage.StorageProvider, bool) {
		return provider, provider != nil
	})
}

// CleanupProviders applies one retention decision to every successful copy of
// a record before deleting its database row. This prevents multi-target tasks
// from leaving stale objects after the first target removes the shared record.
func (s *Service) CleanupProviders(ctx context.Context, task *model.BackupTask, providers map[uint]storage.StorageProvider) (*CleanupResult, error) {
	return s.cleanup(ctx, task, func(targetID uint) (storage.StorageProvider, bool) {
		provider, ok := providers[targetID]
		return provider, ok && provider != nil
	})
}

func (s *Service) cleanup(ctx context.Context, task *model.BackupTask, resolveProvider func(uint) (storage.StorageProvider, bool)) (*CleanupResult, error) {
	if task == nil {
		return nil, fmt.Errorf("backup task is required")
	}
	records, err := s.records.ListSuccessfulByTask(ctx, task.ID)
	if err != nil {
		return nil, fmt.Errorf("list successful records: %w", err)
	}
	var candidates []model.BackupRecord
	if gfsEnabled(task) {
		// GFS 策略：按天/周/月/年分层保留代表性备份，取代简单的天数/数量策略。
		candidates = selectGFSToDelete(records, task.KeepDaily, task.KeepWeekly, task.KeepMonthly, task.KeepYearly)
	} else {
		candidates = selectRecordsToDelete(records, task.RetentionDays, task.MaxBackups, s.now())
	}
	// 差异链保护：保留仍被存活差异依赖的全量，避免删除基线后差异无法恢复。
	candidates = protectDifferentialBases(records, candidates)
	result := &CleanupResult{}
	repositoryProviders := make(map[uint]storage.StorageProvider)
	touchedProviders := make(map[uint]storage.StorageProvider)
	for _, record := range candidates {
		objects, objectErr := cleanupObjectsForRecord(record)
		if objectErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("decode storage copies for record %d failed: %v", record.ID, objectErr))
			continue
		}
		allObjectsDeleted := true
		for _, object := range objects {
			provider, ok := resolveProvider(object.targetID)
			if !ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf("record %d missing storage provider %d for cleanup", record.ID, object.targetID))
				allObjectsDeleted = false
				continue
			}
			if err := provider.Delete(ctx, object.path); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("delete storage object %s from target %d failed: %v", object.path, object.targetID, err))
				allObjectsDeleted = false
				continue
			}
			result.DeletedObjects++
			touchedProviders[object.targetID] = provider
			if record.BackupKind == model.BackupKindRepository {
				repositoryProviders[object.targetID] = provider
			}
		}
		if !allObjectsDeleted {
			continue
		}
		if err := s.records.Delete(ctx, record.ID); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("delete backup record %d failed: %v", record.ID, err))
			continue
		}
		result.DeletedRecords++
	}
	for targetID, provider := range repositoryProviders {
		pruned, pruneErr := backup.NewRepositoryStore(s.repositoryKey).Prune(ctx, provider)
		if pruneErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("prune CDC repository on target %d failed: %v", targetID, pruneErr))
		} else {
			result.DeletedObjects += pruned.DeletedPacks + pruned.DeletedIndexes
		}
	}

	// 清理空目录：收集被删除文件的父目录，尝试移除空目录
	for targetID, provider := range touchedProviders {
		dirCleaner, ok := provider.(storage.StorageDirCleaner)
		if !ok {
			continue
		}
		prefixes := collectDirPrefixes(candidates)
		for _, prefix := range prefixes {
			if err := dirCleaner.RemoveEmptyDirs(ctx, prefix); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("cleanup empty dirs for %s on target %d: %v", prefix, targetID, err))
			}
		}
	}

	return result, nil
}

func cleanupObjectsForRecord(record model.BackupRecord) ([]cleanupObject, error) {
	defaultPath := strings.TrimSpace(record.StoragePath)
	if strings.TrimSpace(record.StorageUploadResults) == "" {
		if defaultPath == "" {
			return nil, nil
		}
		return []cleanupObject{{targetID: record.StorageTargetID, path: defaultPath}}, nil
	}
	var results []storedUploadResult
	if err := json.Unmarshal([]byte(record.StorageUploadResults), &results); err != nil {
		return nil, err
	}
	objects := make([]cleanupObject, 0, len(results))
	seen := make(map[uint]struct{}, len(results))
	for _, result := range results {
		if !strings.EqualFold(strings.TrimSpace(result.Status), model.BackupRecordStatusSuccess) {
			continue
		}
		objectPath := strings.TrimSpace(result.StoragePath)
		if objectPath == "" {
			objectPath = defaultPath
		}
		if objectPath == "" {
			continue
		}
		if _, exists := seen[result.StorageTargetID]; exists {
			continue
		}
		seen[result.StorageTargetID] = struct{}{}
		objects = append(objects, cleanupObject{targetID: result.StorageTargetID, path: objectPath})
	}
	if len(objects) == 0 && defaultPath != "" {
		return nil, fmt.Errorf("successful record has no successful storage copy")
	}
	return objects, nil
}

// protectDifferentialBases 从删除候选中剔除「仍被存活差异依赖的全量」，
// 避免删除基线后其差异备份失去依据、无法恢复。全量仅当其全部差异都已过期/删除时才会被清理。
func protectDifferentialBases(all []model.BackupRecord, candidates []model.BackupRecord) []model.BackupRecord {
	deleting := make(map[uint]struct{}, len(candidates))
	for _, r := range candidates {
		deleting[r.ID] = struct{}{}
	}
	protected := make(map[uint]struct{})
	for _, r := range all {
		if r.BackupKind != model.BackupKindDifferential || r.BaseRecordID == 0 {
			continue
		}
		if _, beingDeleted := deleting[r.ID]; beingDeleted {
			continue // 该差异本身也将被删除，无需保护其基线
		}
		protected[r.BaseRecordID] = struct{}{}
	}
	if len(protected) == 0 {
		return candidates
	}
	filtered := make([]model.BackupRecord, 0, len(candidates))
	for _, r := range candidates {
		if r.BackupKind == model.BackupKindFull {
			if _, keep := protected[r.ID]; keep {
				continue
			}
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func selectRecordsToDelete(records []model.BackupRecord, retentionDays int, maxBackups int, now time.Time) []model.BackupRecord {
	// 保留锁定（法律保留）的记录永不参与清理：先从候选集中剔除，
	// 锁定备份既不被删除，也不占用 maxBackups 轮转名额。
	if hasLocked(records) {
		unlocked := make([]model.BackupRecord, 0, len(records))
		for _, r := range records {
			if !r.Locked {
				unlocked = append(unlocked, r)
			}
		}
		records = unlocked
	}
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

func hasLocked(records []model.BackupRecord) bool {
	for i := range records {
		if records[i].Locked {
			return true
		}
	}
	return false
}

// gfsEnabled 判定任务是否启用 GFS 分层保留（任一层级 > 0）。
func gfsEnabled(task *model.BackupTask) bool {
	return task.KeepDaily > 0 || task.KeepWeekly > 0 || task.KeepMonthly > 0 || task.KeepYearly > 0
}

func recordTime(r *model.BackupRecord) time.Time {
	if r.CompletedAt != nil {
		return *r.CompletedAt
	}
	return r.StartedAt
}

func isoWeekKey(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// selectGFSToDelete 按 GFS（祖父-父-子）策略选出应删除的记录。
//
// 规则：对每个层级（天/周/月/年），在按时间降序排列后，保留最近 keep 个不同周期中
// 每个周期最新的一份备份；各层级保留集合取并集即「保留集」，其余删除。
// 锁定（法律保留）的记录始终排除在删除候选之外。
func selectGFSToDelete(records []model.BackupRecord, daily, weekly, monthly, yearly int) []model.BackupRecord {
	active := make([]model.BackupRecord, 0, len(records))
	for i := range records {
		if !records[i].Locked {
			active = append(active, records[i])
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		return recordTime(&active[i]).After(recordTime(&active[j]))
	})

	keep := make(map[uint]bool, len(active))
	keepTier := func(count int, key func(time.Time) string) {
		if count <= 0 {
			return
		}
		periods := 0
		lastPeriod := ""
		havePrev := false
		for i := range active {
			p := key(recordTime(&active[i]))
			if havePrev && p == lastPeriod {
				continue // 同周期已保留代表（最新一份）
			}
			if periods >= count {
				break // 该层级已保留足够多的周期
			}
			keep[active[i].ID] = true
			lastPeriod = p
			havePrev = true
			periods++
		}
	}
	keepTier(daily, func(t time.Time) string { return t.Format("2006-01-02") })
	keepTier(weekly, isoWeekKey)
	keepTier(monthly, func(t time.Time) string { return t.Format("2006-01") })
	keepTier(yearly, func(t time.Time) string { return t.Format("2006") })

	del := make([]model.BackupRecord, 0)
	for i := range active {
		if !keep[active[i].ID] {
			del = append(del, active[i])
		}
	}
	return del
}
