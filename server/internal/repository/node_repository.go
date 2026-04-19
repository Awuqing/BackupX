package repository

import (
	"context"
	"errors"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type NodeRepository interface {
	List(context.Context) ([]model.Node, error)
	FindByID(context.Context, uint) (*model.Node, error)
	FindByToken(context.Context, string) (*model.Node, error)
	FindLocal(context.Context) (*model.Node, error)
	Create(context.Context, *model.Node) error
	// BatchCreate 在单一事务内批量创建节点，任一失败即全部回滚。
	BatchCreate(ctx context.Context, nodes []*model.Node) error
	Update(context.Context, *model.Node) error
	Delete(context.Context, uint) error
	MarkStaleOffline(ctx context.Context, threshold time.Time) (int64, error)
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
	// 主 token 查询
	err := r.db.WithContext(ctx).Where("token = ?", token).First(&item).Error
	if err == nil {
		return &item, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// 回退：prev_token 且未过期
	now := time.Now().UTC()
	err = r.db.WithContext(ctx).
		Where("prev_token = ? AND prev_token_expires IS NOT NULL AND prev_token_expires > ?", token, now).
		First(&item).Error
	if err != nil {
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

// BatchCreate 在单一事务中批量创建节点。任一记录失败即事务回滚。
// 节点 ID 在事务提交后回填到入参切片元素上。
func (r *GormNodeRepository) BatchCreate(ctx context.Context, nodes []*model.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, n := range nodes {
			if err := tx.Create(n).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormNodeRepository) Update(ctx context.Context, item *model.Node) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *GormNodeRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.Node{}, id).Error
}

// MarkStaleOffline 把最近心跳早于 threshold 的在线远程节点标记为离线。
// 本机节点 (is_local=true) 不受影响，由主程序自己维护 online 状态。
// 返回受影响行数。
func (r *GormNodeRepository) MarkStaleOffline(ctx context.Context, threshold time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.Node{}).
		Where("is_local = ? AND status = ? AND last_seen < ?", false, model.NodeStatusOnline, threshold).
		Update("status", model.NodeStatusOffline)
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
