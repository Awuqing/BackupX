package repository

import (
	"context"
	"errors"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

// RestoreRecordListOptions 恢复记录列表筛选条件。
type RestoreRecordListOptions struct {
	TaskID         *uint
	BackupRecordID *uint
	NodeID         *uint
	Status         string
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}

// RestoreRecordRepository 恢复记录仓储接口。
type RestoreRecordRepository interface {
	Create(ctx context.Context, item *model.RestoreRecord) error
	Update(ctx context.Context, item *model.RestoreRecord) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*model.RestoreRecord, error)
	List(ctx context.Context, options RestoreRecordListOptions) ([]model.RestoreRecord, error)
	Count(ctx context.Context) (int64, error)
}

type GormRestoreRecordRepository struct {
	db *gorm.DB
}

func NewRestoreRecordRepository(db *gorm.DB) *GormRestoreRecordRepository {
	return &GormRestoreRecordRepository{db: db}
}

func (r *GormRestoreRecordRepository) Create(ctx context.Context, item *model.RestoreRecord) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *GormRestoreRecordRepository) Update(ctx context.Context, item *model.RestoreRecord) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *GormRestoreRecordRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.RestoreRecord{}, id).Error
}

func (r *GormRestoreRecordRepository) FindByID(ctx context.Context, id uint) (*model.RestoreRecord, error) {
	var item model.RestoreRecord
	if err := r.db.WithContext(ctx).
		Preload("Task").
		Preload("BackupRecord").
		First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormRestoreRecordRepository) List(ctx context.Context, options RestoreRecordListOptions) ([]model.RestoreRecord, error) {
	query := r.db.WithContext(ctx).
		Model(&model.RestoreRecord{}).
		Preload("Task").
		Preload("BackupRecord").
		Order("started_at desc")
	if options.TaskID != nil {
		query = query.Where("task_id = ?", *options.TaskID)
	}
	if options.BackupRecordID != nil {
		query = query.Where("backup_record_id = ?", *options.BackupRecordID)
	}
	if options.NodeID != nil {
		query = query.Where("node_id = ?", *options.NodeID)
	}
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}
	if options.DateFrom != nil {
		query = query.Where("started_at >= ?", options.DateFrom.UTC())
	}
	if options.DateTo != nil {
		query = query.Where("started_at <= ?", options.DateTo.UTC())
	}
	if options.Limit > 0 {
		query = query.Limit(options.Limit)
	}
	if options.Offset > 0 {
		query = query.Offset(options.Offset)
	}
	var items []model.RestoreRecord
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormRestoreRecordRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.RestoreRecord{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
