package repository

import (
	"context"
	"errors"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type UserRepository interface {
	Count(context.Context) (int64, error)
	CountByRole(context.Context, string) (int64, error)
	Create(context.Context, *model.User) error
	Update(context.Context, *model.User) error
	Delete(context.Context, uint) error
	List(context.Context) ([]model.User, error)
	FindByUsername(context.Context, string) (*model.User, error)
	FindByID(context.Context, uint) (*model.User, error)
}

type GormUserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountByRole 按角色统计启用（非 disabled）用户数。用于防止删除最后一个 admin。
func (r *GormUserRepository) CountByRole(ctx context.Context, role string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("role = ? AND disabled = ?", role, false).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// List 按创建时间升序返回所有用户。
func (r *GormUserRepository) List(ctx context.Context) ([]model.User, error) {
	var items []model.User
	if err := r.db.WithContext(ctx).Order("created_at asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Delete 物理删除用户。调用方应先在 service 层检查最后 admin。
func (r *GormUserRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, id).Error
}

func (r *GormUserRepository) Create(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *GormUserRepository) Update(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

func (r *GormUserRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *GormUserRepository) FindByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
