package repository

import (
	"context"
	"errors"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

// AgentCommandRepository 维护 Agent 命令队列。
type AgentCommandRepository interface {
	Create(ctx context.Context, cmd *model.AgentCommand) error
	FindByID(ctx context.Context, id uint) (*model.AgentCommand, error)
	// ClaimPending 以原子方式把该节点一条 pending 命令置为 dispatched，
	// 并返回领取到的命令。无命令时返回 (nil, nil)。
	ClaimPending(ctx context.Context, nodeID uint) (*model.AgentCommand, error)
	Update(ctx context.Context, cmd *model.AgentCommand) error
	// MarkStaleTimeout 把 dispatched 状态但超时未完成的命令标记为 timeout。
	// 返回被标记的行数。不返回具体命令（供背景监控简单调用）。
	MarkStaleTimeout(ctx context.Context, threshold time.Time) (int64, error)
	// ListStaleDispatched 列出 dispatched 但已超时、尚未被标记的命令。
	// 调用方需要把它们逐一标记 timeout 并联动关联记录状态。
	ListStaleDispatched(ctx context.Context, threshold time.Time) ([]model.AgentCommand, error)
	// ListPendingByNode 列出某节点下的所有 pending/dispatched 命令。
	// 用于删除节点或节点离线时的清理。
	ListPendingByNode(ctx context.Context, nodeID uint) ([]model.AgentCommand, error)
}

type GormAgentCommandRepository struct {
	db *gorm.DB
}

func NewAgentCommandRepository(db *gorm.DB) *GormAgentCommandRepository {
	return &GormAgentCommandRepository{db: db}
}

func (r *GormAgentCommandRepository) Create(ctx context.Context, cmd *model.AgentCommand) error {
	return r.db.WithContext(ctx).Create(cmd).Error
}

func (r *GormAgentCommandRepository) FindByID(ctx context.Context, id uint) (*model.AgentCommand, error) {
	var item model.AgentCommand
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// ClaimPending 使用 UPDATE...WHERE id=(SELECT...) 的两步方式实现原子领取。
// SQLite 不支持 SELECT FOR UPDATE，这里用事务 + 乐观锁。
func (r *GormAgentCommandRepository) ClaimPending(ctx context.Context, nodeID uint) (*model.AgentCommand, error) {
	var claimed *model.AgentCommand
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item model.AgentCommand
		err := tx.Where("node_id = ? AND status = ?", nodeID, model.AgentCommandStatusPending).
			Order("id asc").First(&item).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		now := time.Now().UTC()
		result := tx.Model(&model.AgentCommand{}).
			Where("id = ? AND status = ?", item.ID, model.AgentCommandStatusPending).
			Updates(map[string]any{
				"status":        model.AgentCommandStatusDispatched,
				"dispatched_at": &now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// 被其它 worker 抢占，放弃
			return nil
		}
		item.Status = model.AgentCommandStatusDispatched
		item.DispatchedAt = &now
		claimed = &item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func (r *GormAgentCommandRepository) Update(ctx context.Context, cmd *model.AgentCommand) error {
	return r.db.WithContext(ctx).Save(cmd).Error
}

func (r *GormAgentCommandRepository) MarkStaleTimeout(ctx context.Context, threshold time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.AgentCommand{}).
		Where("status = ? AND dispatched_at < ?", model.AgentCommandStatusDispatched, threshold).
		Updates(map[string]any{
			"status":        model.AgentCommandStatusTimeout,
			"error_message": "agent did not report result before timeout",
		})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// ListStaleDispatched 列出 dispatched 但 dispatched_at 早于 threshold 的命令。
func (r *GormAgentCommandRepository) ListStaleDispatched(ctx context.Context, threshold time.Time) ([]model.AgentCommand, error) {
	var items []model.AgentCommand
	if err := r.db.WithContext(ctx).
		Where("status = ? AND dispatched_at < ?", model.AgentCommandStatusDispatched, threshold).
		Order("id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListPendingByNode 列出某节点下所有待执行（pending 或 dispatched）命令。
func (r *GormAgentCommandRepository) ListPendingByNode(ctx context.Context, nodeID uint) ([]model.AgentCommand, error) {
	var items []model.AgentCommand
	if err := r.db.WithContext(ctx).
		Where("node_id = ? AND status IN ?", nodeID, []string{
			model.AgentCommandStatusPending,
			model.AgentCommandStatusDispatched,
		}).
		Order("id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
