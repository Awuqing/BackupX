package service

import (
	"context"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
)

type RetentionService struct {
	records         repository.BackupRecordRepository
	storageTargets  repository.StorageTargetRepository
	storageRegistry *storage.Registry
	cipher          *codec.ConfigCipher
}

func NewRetentionService(records repository.BackupRecordRepository, storageTargets repository.StorageTargetRepository, storageRegistry *storage.Registry, cipher *codec.ConfigCipher) *RetentionService {
	return &RetentionService{records: records, storageTargets: storageTargets, storageRegistry: storageRegistry, cipher: cipher}
}

func (s *RetentionService) Apply(ctx context.Context, task *model.BackupTask) error {
	if task == nil || (task.RetentionDays <= 0 && task.MaxBackups <= 0) {
		return nil
	}
	items, err := s.records.ListSuccessfulByTask(ctx, task.ID)
	if err != nil {
		return err
	}
	removeSet := make(map[uint]model.BackupRecord)
	if task.RetentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -task.RetentionDays)
		for _, item := range items {
			if item.CompletedAt != nil && item.CompletedAt.Before(cutoff) {
				removeSet[item.ID] = item
			}
		}
	}
	if task.MaxBackups > 0 {
		kept := 0
		for _, item := range items {
			if _, marked := removeSet[item.ID]; marked {
				continue
			}
			kept++
			if kept > task.MaxBackups {
				removeSet[item.ID] = item
			}
		}
	}
	if len(removeSet) == 0 {
		return nil
	}
	provider, _, err := buildStorageProviderFromRepos(ctx, task.StorageTargetID, s.storageTargets, s.storageRegistry, s.cipher)
	if err != nil {
		return err
	}
	for _, item := range removeSet {
		if item.StoragePath != "" {
			if err := provider.Delete(ctx, item.StoragePath); err != nil {
				return err
			}
		}
		if err := s.records.Delete(ctx, item.ID); err != nil {
			return err
		}
	}
	return nil
}
