package repository

import (
	"context"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type AuditLogListOptions struct {
	Category string
	Action   string
	Username string
	TargetID string
	Keyword  string // 模糊匹配 detail / target_name
	DateFrom *time.Time
	DateTo   *time.Time
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
	ListAll(ctx context.Context, opts AuditLogListOptions) ([]model.AuditLog, error)
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
	query := r.buildQuery(opts)
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

// ListAll 导出专用：不分页返回所有匹配记录（上限 10k 防爆）。
func (r *gormAuditLogRepository) ListAll(_ context.Context, opts AuditLogListOptions) ([]model.AuditLog, error) {
	query := r.buildQuery(opts)
	const maxExportRows = 10000
	var items []model.AuditLog
	if err := query.Order("created_at DESC").Limit(maxExportRows).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// buildQuery 统一构造带筛选条件的查询。
func (r *gormAuditLogRepository) buildQuery(opts AuditLogListOptions) *gorm.DB {
	query := r.db.Model(&model.AuditLog{})
	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.Action != "" {
		query = query.Where("action = ?", opts.Action)
	}
	if opts.Username != "" {
		query = query.Where("username = ?", opts.Username)
	}
	if opts.TargetID != "" {
		query = query.Where("target_id = ?", opts.TargetID)
	}
	if opts.Keyword != "" {
		pattern := "%" + opts.Keyword + "%"
		query = query.Where("detail LIKE ? OR target_name LIKE ?", pattern, pattern)
	}
	if opts.DateFrom != nil {
		query = query.Where("created_at >= ?", opts.DateFrom.UTC())
	}
	if opts.DateTo != nil {
		query = query.Where("created_at <= ?", opts.DateTo.UTC())
	}
	return query
}
