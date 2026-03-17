package repository

import (
	"context"
	"errors"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type OAuthSessionRepository interface {
	Create(context.Context, *model.OAuthSession) error
	Update(context.Context, *model.OAuthSession) error
	FindByState(context.Context, string) (*model.OAuthSession, error)
	DeleteExpired(context.Context, time.Time) error
}

type GormOAuthSessionRepository struct {
	db *gorm.DB
}

func NewOAuthSessionRepository(db *gorm.DB) *GormOAuthSessionRepository {
	return &GormOAuthSessionRepository{db: db}
}

func (r *GormOAuthSessionRepository) Create(ctx context.Context, item *model.OAuthSession) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *GormOAuthSessionRepository) Update(ctx context.Context, item *model.OAuthSession) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *GormOAuthSessionRepository) FindByState(ctx context.Context, state string) (*model.OAuthSession, error) {
	var item model.OAuthSession
	if err := r.db.WithContext(ctx).Where("state = ?", state).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormOAuthSessionRepository) DeleteExpired(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).Where("expires_at <= ?", before).Delete(&model.OAuthSession{}).Error
}
