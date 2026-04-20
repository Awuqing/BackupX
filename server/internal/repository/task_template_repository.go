package repository

import (
	"context"
	"errors"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type TaskTemplateRepository interface {
	Create(ctx context.Context, template *model.TaskTemplate) error
	Update(ctx context.Context, template *model.TaskTemplate) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*model.TaskTemplate, error)
	FindByName(ctx context.Context, name string) (*model.TaskTemplate, error)
	List(ctx context.Context) ([]model.TaskTemplate, error)
}

type GormTaskTemplateRepository struct {
	db *gorm.DB
}

func NewTaskTemplateRepository(db *gorm.DB) *GormTaskTemplateRepository {
	return &GormTaskTemplateRepository{db: db}
}

func (r *GormTaskTemplateRepository) Create(ctx context.Context, t *model.TaskTemplate) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *GormTaskTemplateRepository) Update(ctx context.Context, t *model.TaskTemplate) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *GormTaskTemplateRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.TaskTemplate{}, id).Error
}

func (r *GormTaskTemplateRepository) FindByID(ctx context.Context, id uint) (*model.TaskTemplate, error) {
	var item model.TaskTemplate
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormTaskTemplateRepository) FindByName(ctx context.Context, name string) (*model.TaskTemplate, error) {
	var item model.TaskTemplate
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormTaskTemplateRepository) List(ctx context.Context) ([]model.TaskTemplate, error) {
	var items []model.TaskTemplate
	if err := r.db.WithContext(ctx).Order("name asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
