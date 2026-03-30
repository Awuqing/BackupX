package repository

import (
	"context"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type AuditLogListOptions struct {
	Category string
	Limit    int
	Offset   int
}

type AuditLogListResult struct {
	Items []model.AuditLog `json:"items"`
	Total int64            `json:"total"`
}

type AuditLogRepository interface {
	Create(ctx context.Context, log *model.AuditLog) error
	List(ctx context.Context, opts AuditLogListOptions) (*AuditLogListResult, error)
}

type gormAuditLogRepository struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &gormAuditLogRepository{db: db}
}

func (r *gormAuditLogRepository) Create(_ context.Context, log *model.AuditLog) error {
	return r.db.Create(log).Error
}

func (r *gormAuditLogRepository) List(_ context.Context, opts AuditLogListOptions) (*AuditLogListResult, error) {
	query := r.db.Model(&model.AuditLog{})
	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	var items []model.AuditLog
	if err := query.Order("created_at DESC").Offset(opts.Offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return &AuditLogListResult{Items: items, Total: total}, nil
}
