package repository

import (
	"context"
	"errors"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type StorageTargetRepository interface {
	List(context.Context) ([]model.StorageTarget, error)
	FindByID(context.Context, uint) (*model.StorageTarget, error)
	FindByName(context.Context, string) (*model.StorageTarget, error)
	Create(context.Context, *model.StorageTarget) error
	Update(context.Context, *model.StorageTarget) error
	Delete(context.Context, uint) error
}

type GormStorageTargetRepository struct {
	db *gorm.DB
}

func NewStorageTargetRepository(db *gorm.DB) *GormStorageTargetRepository {
	return &GormStorageTargetRepository{db: db}
}

func (r *GormStorageTargetRepository) List(ctx context.Context) ([]model.StorageTarget, error) {
	var items []model.StorageTarget
	if err := r.db.WithContext(ctx).Order("updated_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormStorageTargetRepository) FindByID(ctx context.Context, id uint) (*model.StorageTarget, error) {
	var item model.StorageTarget
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormStorageTargetRepository) FindByName(ctx context.Context, name string) (*model.StorageTarget, error) {
	var item model.StorageTarget
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormStorageTargetRepository) Create(ctx context.Context, item *model.StorageTarget) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *GormStorageTargetRepository) Update(ctx context.Context, item *model.StorageTarget) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *GormStorageTargetRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.StorageTarget{}, id).Error
}
