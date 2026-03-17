package repository

import (
	"context"
	"errors"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type NodeRepository interface {
	List(context.Context) ([]model.Node, error)
	FindByID(context.Context, uint) (*model.Node, error)
	FindByToken(context.Context, string) (*model.Node, error)
	FindLocal(context.Context) (*model.Node, error)
	Create(context.Context, *model.Node) error
	Update(context.Context, *model.Node) error
	Delete(context.Context, uint) error
}

type GormNodeRepository struct {
	db *gorm.DB
}

func NewNodeRepository(db *gorm.DB) *GormNodeRepository {
	return &GormNodeRepository{db: db}
}

func (r *GormNodeRepository) List(ctx context.Context) ([]model.Node, error) {
	var items []model.Node
	if err := r.db.WithContext(ctx).Order("is_local desc, updated_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormNodeRepository) FindByID(ctx context.Context, id uint) (*model.Node, error) {
	var item model.Node
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormNodeRepository) FindByToken(ctx context.Context, token string) (*model.Node, error) {
	var item model.Node
	if err := r.db.WithContext(ctx).Where("token = ?", token).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormNodeRepository) FindLocal(ctx context.Context) (*model.Node, error) {
	var item model.Node
	if err := r.db.WithContext(ctx).Where("is_local = ?", true).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormNodeRepository) Create(ctx context.Context, item *model.Node) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *GormNodeRepository) Update(ctx context.Context, item *model.Node) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *GormNodeRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.Node{}, id).Error
}
