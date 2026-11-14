package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"backupx/server/internal/backup"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// Each log archive contains all binlogs since its full baseline; no intermediate
// record is required for recovery, and the existing base retention lock applies.
func resolveMySQLBase(ctx context.Context, records repository.BackupRecordRepository, task *model.BackupTask, nodeID uint, now time.Time) (uint, *backup.MySQLCheckpoint, error) {
	if task.Type != model.BackupTaskTypeMySQL || !backup.MySQLPITREnabled(task.ExtraConfig) {
		return 0, nil, nil
	}
	items, err := records.ListSuccessfulByTask(ctx, task.ID)
	if err != nil {
		return 0, nil, err
	}
	for _, item := range items {
		if item.BackupKind != model.BackupKindFull || item.MySQLCheckpoint == "" || item.NodeID != nodeID {
			continue
		}
		var checkpoint backup.MySQLCheckpoint
		if err := json.Unmarshal([]byte(item.MySQLCheckpoint), &checkpoint); err != nil {
			return 0, nil, fmt.Errorf("decode MySQL baseline: %w", err)
		}
		if err := checkpoint.Validate(); err != nil {
			return 0, nil, err
		}
		if !checkpoint.Full {
			return 0, nil, fmt.Errorf("invalid MySQL full baseline")
		}
		if now.Sub(item.StartedAt) >= backup.MySQLFullInterval(task.ExtraConfig) || checkpoint.Source != backup.MySQLSource(backup.DatabaseSpec{Host: task.DBHost, Port: task.DBPort}) {
			return 0, nil, nil
		}
		return item.ID, &checkpoint, nil
	}
	return 0, nil, nil
}
