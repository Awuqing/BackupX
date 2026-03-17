package repository

import (
	"context"
	"errors"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type BackupTaskListOptions struct {
	Type    string
	Enabled *bool
}

type BackupTaskRepository interface {
	List(context.Context, BackupTaskListOptions) ([]model.BackupTask, error)
	FindByID(context.Context, uint) (*model.BackupTask, error)
	FindByName(context.Context, string) (*model.BackupTask, error)
	ListSchedulable(context.Context) ([]model.BackupTask, error)
	Count(context.Context) (int64, error)
	CountEnabled(context.Context) (int64, error)
	CountByStorageTargetID(context.Context, uint) (int64, error)
	Create(context.Context, *model.BackupTask) error
	Update(context.Context, *model.BackupTask) error
	Delete(context.Context, uint) error
}

type GormBackupTaskRepository struct {
	db *gorm.DB
}

func NewBackupTaskRepository(db *gorm.DB) *GormBackupTaskRepository {
	return &GormBackupTaskRepository{db: db}
}

func (r *GormBackupTaskRepository) List(ctx context.Context, options BackupTaskListOptions) ([]model.BackupTask, error) {
	query := r.db.WithContext(ctx).Model(&model.BackupTask{}).Preload("StorageTarget").Order("updated_at desc")
	if options.Type != "" {
		query = query.Where("type = ?", options.Type)
	}
	if options.Enabled != nil {
		query = query.Where("enabled = ?", *options.Enabled)
	}
	var items []model.BackupTask
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormBackupTaskRepository) FindByID(ctx context.Context, id uint) (*model.BackupTask, error) {
	var item model.BackupTask
	if err := r.db.WithContext(ctx).Preload("StorageTarget").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormBackupTaskRepository) FindByName(ctx context.Context, name string) (*model.BackupTask, error) {
	var item model.BackupTask
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormBackupTaskRepository) ListSchedulable(ctx context.Context) ([]model.BackupTask, error) {
	var items []model.BackupTask
	if err := r.db.WithContext(ctx).Preload("StorageTarget").Where("enabled = ? AND cron_expr <> ''", true).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormBackupTaskRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupTask{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormBackupTaskRepository) CountEnabled(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupTask{}).Where("enabled = ?", true).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormBackupTaskRepository) CountByStorageTargetID(ctx context.Context, storageTargetID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupTask{}).Where("storage_target_id = ?", storageTargetID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormBackupTaskRepository) Create(ctx context.Context, item *model.BackupTask) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *GormBackupTaskRepository) Update(ctx context.Context, item *model.BackupTask) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *GormBackupTaskRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.BackupTask{}, id).Error
}
