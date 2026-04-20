package repository

import (
	"context"
	"errors"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type ApiKeyRepository interface {
	Create(ctx context.Context, key *model.ApiKey) error
	Update(ctx context.Context, key *model.ApiKey) error
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*model.ApiKey, error)
	FindByHash(ctx context.Context, hash string) (*model.ApiKey, error)
	List(ctx context.Context) ([]model.ApiKey, error)
	MarkUsed(ctx context.Context, id uint, at time.Time) error
}

type GormApiKeyRepository struct {
	db *gorm.DB
}

func NewApiKeyRepository(db *gorm.DB) *GormApiKeyRepository {
	return &GormApiKeyRepository{db: db}
}

func (r *GormApiKeyRepository) Create(ctx context.Context, key *model.ApiKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *GormApiKeyRepository) Update(ctx context.Context, key *model.ApiKey) error {
	return r.db.WithContext(ctx).Save(key).Error
}

func (r *GormApiKeyRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.ApiKey{}, id).Error
}

func (r *GormApiKeyRepository) FindByID(ctx context.Context, id uint) (*model.ApiKey, error) {
	var item model.ApiKey
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormApiKeyRepository) FindByHash(ctx context.Context, hash string) (*model.ApiKey, error) {
	var item model.ApiKey
	if err := r.db.WithContext(ctx).Where("key_hash = ?", hash).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormApiKeyRepository) List(ctx context.Context) ([]model.ApiKey, error) {
	var items []model.ApiKey
	if err := r.db.WithContext(ctx).Order("created_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// MarkUsed 更新最近使用时间。写入失败不应阻断认证主流程，调用方需忽略错误。
func (r *GormApiKeyRepository) MarkUsed(ctx context.Context, id uint, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.ApiKey{}).
		Where("id = ?", id).
		Update("last_used_at", at).Error
}
