package repository

import (
	"context"
	"errors"

	"backupx/server/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SystemConfigRepository interface {
	GetByKey(context.Context, string) (*model.SystemConfig, error)
	List(context.Context) ([]model.SystemConfig, error)
	Upsert(context.Context, *model.SystemConfig) error
}

type GormSystemConfigRepository struct {
	db *gorm.DB
}

func NewSystemConfigRepository(db *gorm.DB) *GormSystemConfigRepository {
	return &GormSystemConfigRepository{db: db}
}

func (r *GormSystemConfigRepository) GetByKey(ctx context.Context, key string) (*model.SystemConfig, error) {
	var item model.SystemConfig
	if err := r.db.WithContext(ctx).Where("key = ?", key).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormSystemConfigRepository) List(ctx context.Context) ([]model.SystemConfig, error) {
	var items []model.SystemConfig
	if err := r.db.WithContext(ctx).Order("key ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormSystemConfigRepository) Upsert(ctx context.Context, item *model.SystemConfig) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "encrypted", "updated_at"}),
	}).Create(item).Error
}
