package repository

import (
	"context"
	"errors"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type NotificationRepository interface {
	List(context.Context) ([]model.Notification, error)
	ListEnabledForEvent(context.Context, bool) ([]model.Notification, error)
	FindByID(context.Context, uint) (*model.Notification, error)
	FindByName(context.Context, string) (*model.Notification, error)
	Create(context.Context, *model.Notification) error
	Update(context.Context, *model.Notification) error
	Delete(context.Context, uint) error
}

type GormNotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *GormNotificationRepository {
	return &GormNotificationRepository{db: db}
}

func (r *GormNotificationRepository) List(ctx context.Context) ([]model.Notification, error) {
	var items []model.Notification
	if err := r.db.WithContext(ctx).Order("updated_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormNotificationRepository) ListEnabledForEvent(ctx context.Context, success bool) ([]model.Notification, error) {
	query := r.db.WithContext(ctx).Model(&model.Notification{}).Where("enabled = ?", true)
	if success {
		query = query.Where("on_success = ?", true)
	} else {
		query = query.Where("on_failure = ?", true)
	}
	var items []model.Notification
	if err := query.Order("updated_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormNotificationRepository) FindByID(ctx context.Context, id uint) (*model.Notification, error) {
	var item model.Notification
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormNotificationRepository) FindByName(ctx context.Context, name string) (*model.Notification, error) {
	var item model.Notification
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormNotificationRepository) Create(ctx context.Context, item *model.Notification) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *GormNotificationRepository) Update(ctx context.Context, item *model.Notification) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *GormNotificationRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.Notification{}, id).Error
}
