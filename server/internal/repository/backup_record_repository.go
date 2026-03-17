package repository

import (
	"context"
	"errors"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type BackupRecordListOptions struct {
	TaskID   *uint
	Status   string
	DateFrom *time.Time
	DateTo   *time.Time
	Limit    int
	Offset   int
}

type BackupTimelinePoint struct {
	Date    string `json:"date"`
	Total   int64  `json:"total"`
	Success int64  `json:"success"`
	Failed  int64  `json:"failed"`
}

type BackupStorageUsageItem struct {
	StorageTargetID uint  `json:"storageTargetId"`
	TotalSize       int64 `json:"totalSize"`
}

type BackupRecordRepository interface {
	List(context.Context, BackupRecordListOptions) ([]model.BackupRecord, error)
	FindByID(context.Context, uint) (*model.BackupRecord, error)
	Create(context.Context, *model.BackupRecord) error
	Update(context.Context, *model.BackupRecord) error
	Delete(context.Context, uint) error
	ListRecent(context.Context, int) ([]model.BackupRecord, error)
	ListSuccessfulByTask(context.Context, uint) ([]model.BackupRecord, error)
	Count(context.Context) (int64, error)
	CountSince(context.Context, time.Time) (int64, error)
	CountSuccessSince(context.Context, time.Time) (int64, error)
	SumFileSize(context.Context) (int64, error)
	TimelineSince(context.Context, time.Time) ([]BackupTimelinePoint, error)
	StorageUsage(context.Context) ([]BackupStorageUsageItem, error)
}

type GormBackupRecordRepository struct {
	db *gorm.DB
}

func NewBackupRecordRepository(db *gorm.DB) *GormBackupRecordRepository {
	return &GormBackupRecordRepository{db: db}
}

func (r *GormBackupRecordRepository) List(ctx context.Context, options BackupRecordListOptions) ([]model.BackupRecord, error) {
	query := r.db.WithContext(ctx).Model(&model.BackupRecord{}).Preload("Task").Preload("Task.StorageTarget").Order("started_at desc")
	if options.TaskID != nil {
		query = query.Where("task_id = ?", *options.TaskID)
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
	var items []model.BackupRecord
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormBackupRecordRepository) FindByID(ctx context.Context, id uint) (*model.BackupRecord, error) {
	var item model.BackupRecord
	if err := r.db.WithContext(ctx).Preload("Task").Preload("Task.StorageTarget").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormBackupRecordRepository) Create(ctx context.Context, item *model.BackupRecord) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *GormBackupRecordRepository) Update(ctx context.Context, item *model.BackupRecord) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *GormBackupRecordRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.BackupRecord{}, id).Error
}

func (r *GormBackupRecordRepository) ListRecent(ctx context.Context, limit int) ([]model.BackupRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	var items []model.BackupRecord
	if err := r.db.WithContext(ctx).Preload("Task").Preload("Task.StorageTarget").Order("started_at desc").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormBackupRecordRepository) ListSuccessfulByTask(ctx context.Context, taskID uint) ([]model.BackupRecord, error) {
	var items []model.BackupRecord
	if err := r.db.WithContext(ctx).Where("task_id = ? AND status = ?", taskID, "success").Order("completed_at desc, id desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormBackupRecordRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupRecord{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormBackupRecordRepository) CountSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupRecord{}).Where("started_at >= ?", since.UTC()).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormBackupRecordRepository) CountSuccessSince(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupRecord{}).Where("started_at >= ? AND status = ?", since.UTC(), "success").Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormBackupRecordRepository) SumFileSize(ctx context.Context) (int64, error) {
	var sum int64
	if err := r.db.WithContext(ctx).Model(&model.BackupRecord{}).Select("COALESCE(SUM(file_size), 0)").Scan(&sum).Error; err != nil {
		return 0, err
	}
	return sum, nil
}

func (r *GormBackupRecordRepository) TimelineSince(ctx context.Context, since time.Time) ([]BackupTimelinePoint, error) {
	var items []BackupTimelinePoint
	query := `
		SELECT
			strftime('%Y-%m-%d', started_at) AS date,
			COUNT(*) AS total,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS success,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed
		FROM backup_records
		WHERE started_at >= ?
		GROUP BY strftime('%Y-%m-%d', started_at)
		ORDER BY date ASC
	`
	if err := r.db.WithContext(ctx).Raw(query, since.UTC()).Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormBackupRecordRepository) StorageUsage(ctx context.Context) ([]BackupStorageUsageItem, error) {
	var items []BackupStorageUsageItem
	if err := r.db.WithContext(ctx).Model(&model.BackupRecord{}).Select("storage_target_id, COALESCE(SUM(file_size), 0) AS total_size").Group("storage_target_id").Order("storage_target_id asc").Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
