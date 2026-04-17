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
	CountByNodeID(context.Context, uint) (int64, error)
	ListByNodeID(context.Context, uint) ([]model.BackupTask, error)
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
	query := r.db.WithContext(ctx).Model(&model.BackupTask{}).Preload("StorageTarget").Preload("StorageTargets").Order("updated_at desc")
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
	if err := r.db.WithContext(ctx).Preload("StorageTarget").Preload("StorageTargets").First(&item, id).Error; err != nil {
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
	if err := r.db.WithContext(ctx).Preload("StorageTarget").Preload("StorageTargets").Where("enabled = ? AND cron_expr <> ''", true).Order("id asc").Find(&items).Error; err != nil {
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
	if err := r.db.WithContext(ctx).Model(&model.BackupTaskStorageTarget{}).Where("storage_target_id = ?", storageTargetID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountByNodeID 统计绑定到指定节点的任务数。用于删除节点前的引用检查。
func (r *GormBackupTaskRepository) CountByNodeID(ctx context.Context, nodeID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupTask{}).Where("node_id = ?", nodeID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ListByNodeID 列出绑定到指定节点的任务。用于 Agent 拉取本节点待执行任务。
func (r *GormBackupTaskRepository) ListByNodeID(ctx context.Context, nodeID uint) ([]model.BackupTask, error) {
	var items []model.BackupTask
	if err := r.db.WithContext(ctx).Preload("StorageTarget").Preload("StorageTargets").Where("node_id = ?", nodeID).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormBackupTaskRepository) Create(ctx context.Context, item *model.BackupTask) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return err
	}
	return r.syncStorageTargets(ctx, item)
}

func (r *GormBackupTaskRepository) Update(ctx context.Context, item *model.BackupTask) error {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return err
	}
	if len(item.StorageTargets) > 0 {
		return r.db.WithContext(ctx).Model(item).Association("StorageTargets").Replace(item.StorageTargets)
	}
	return nil
}

// syncStorageTargets 确保中间表数据一致：优先使用 StorageTargets，回退到 StorageTargetID
func (r *GormBackupTaskRepository) syncStorageTargets(ctx context.Context, item *model.BackupTask) error {
	targets := item.StorageTargets
	if len(targets) == 0 && item.StorageTargetID > 0 {
		targets = []model.StorageTarget{{ID: item.StorageTargetID}}
	}
	if len(targets) > 0 {
		return r.db.WithContext(ctx).Model(item).Association("StorageTargets").Replace(targets)
	}
	return nil
}

func (r *GormBackupTaskRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.BackupTask{}, id).Error
}
