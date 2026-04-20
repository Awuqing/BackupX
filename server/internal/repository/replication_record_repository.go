package repository

import (
	"context"
	"errors"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type ReplicationRecordListOptions struct {
	TaskID         *uint
	BackupRecordID *uint
	DestTargetID   *uint
	Status         string
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}

type ReplicationRecordRepository interface {
	Create(ctx context.Context, record *model.ReplicationRecord) error
	Update(ctx context.Context, record *model.ReplicationRecord) error
	FindByID(ctx context.Context, id uint) (*model.ReplicationRecord, error)
	List(ctx context.Context, opts ReplicationRecordListOptions) ([]model.ReplicationRecord, error)
	Count(ctx context.Context) (int64, error)
}

type GormReplicationRecordRepository struct {
	db *gorm.DB
}

func NewReplicationRecordRepository(db *gorm.DB) *GormReplicationRecordRepository {
	return &GormReplicationRecordRepository{db: db}
}

func (r *GormReplicationRecordRepository) Create(ctx context.Context, item *model.ReplicationRecord) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *GormReplicationRecordRepository) Update(ctx context.Context, item *model.ReplicationRecord) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *GormReplicationRecordRepository) FindByID(ctx context.Context, id uint) (*model.ReplicationRecord, error) {
	var item model.ReplicationRecord
	if err := r.db.WithContext(ctx).
		Preload("BackupRecord").
		Preload("SourceTarget").
		Preload("DestTarget").
		First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormReplicationRecordRepository) List(ctx context.Context, opts ReplicationRecordListOptions) ([]model.ReplicationRecord, error) {
	query := r.db.WithContext(ctx).
		Model(&model.ReplicationRecord{}).
		Preload("BackupRecord").
		Preload("SourceTarget").
		Preload("DestTarget").
		Order("started_at desc")
	if opts.TaskID != nil {
		query = query.Where("task_id = ?", *opts.TaskID)
	}
	if opts.BackupRecordID != nil {
		query = query.Where("backup_record_id = ?", *opts.BackupRecordID)
	}
	if opts.DestTargetID != nil {
		query = query.Where("dest_target_id = ?", *opts.DestTargetID)
	}
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	}
	if opts.DateFrom != nil {
		query = query.Where("started_at >= ?", opts.DateFrom.UTC())
	}
	if opts.DateTo != nil {
		query = query.Where("started_at <= ?", opts.DateTo.UTC())
	}
	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}
	if opts.Offset > 0 {
		query = query.Offset(opts.Offset)
	}
	var items []model.ReplicationRecord
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormReplicationRecordRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.ReplicationRecord{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
