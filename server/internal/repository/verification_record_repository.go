package repository

import (
	"context"
	"errors"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

// VerificationRecordListOptions 验证记录列表筛选条件。
type VerificationRecordListOptions struct {
	TaskID         *uint
	BackupRecordID *uint
	Status         string
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}

type VerificationRecordRepository interface {
	Create(ctx context.Context, item *model.VerificationRecord) error
	Update(ctx context.Context, item *model.VerificationRecord) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*model.VerificationRecord, error)
	List(ctx context.Context, options VerificationRecordListOptions) ([]model.VerificationRecord, error)
	FindLatestByTask(ctx context.Context, taskID uint) (*model.VerificationRecord, error)
	Count(ctx context.Context) (int64, error)
}

type GormVerificationRecordRepository struct {
	db *gorm.DB
}

func NewVerificationRecordRepository(db *gorm.DB) *GormVerificationRecordRepository {
	return &GormVerificationRecordRepository{db: db}
}

func (r *GormVerificationRecordRepository) Create(ctx context.Context, item *model.VerificationRecord) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *GormVerificationRecordRepository) Update(ctx context.Context, item *model.VerificationRecord) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *GormVerificationRecordRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.VerificationRecord{}, id).Error
}

func (r *GormVerificationRecordRepository) FindByID(ctx context.Context, id uint) (*model.VerificationRecord, error) {
	var item model.VerificationRecord
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

func (r *GormVerificationRecordRepository) List(ctx context.Context, options VerificationRecordListOptions) ([]model.VerificationRecord, error) {
	query := r.db.WithContext(ctx).
		Model(&model.VerificationRecord{}).
		Preload("Task").
		Preload("BackupRecord").
		Order("started_at desc")
	if options.TaskID != nil {
		query = query.Where("task_id = ?", *options.TaskID)
	}
	if options.BackupRecordID != nil {
		query = query.Where("backup_record_id = ?", *options.BackupRecordID)
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
	var items []model.VerificationRecord
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormVerificationRecordRepository) FindLatestByTask(ctx context.Context, taskID uint) (*model.VerificationRecord, error) {
	var item model.VerificationRecord
	if err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("started_at desc").
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormVerificationRecordRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.VerificationRecord{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
