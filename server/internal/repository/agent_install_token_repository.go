package repository

import (
	"context"
	"errors"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

// AgentInstallTokenRepository 一次性安装令牌仓储。
type AgentInstallTokenRepository interface {
	Create(ctx context.Context, t *model.AgentInstallToken) error
	// FindByToken 按 token 字符串查询（不过滤状态），用于管理工具或审计场景。
	FindByToken(ctx context.Context, token string) (*model.AgentInstallToken, error)
	// FindValidByToken 查询且要求 consumed_at IS NULL 且 expires_at > now，
	// 适用于 compose 端点预检 Mode 等"只读不消费但需有效"的场景。
	FindValidByToken(ctx context.Context, token string) (*model.AgentInstallToken, error)
	// ConsumeByToken 原子消费：仅当 token 存在、未过期、未消费时成功，返回消费后的记录。
	// 其它情况（不存在/已过期/已消费）一律返回 (nil, nil)。
	ConsumeByToken(ctx context.Context, token string) (*model.AgentInstallToken, error)
	// DeleteExpiredBefore 硬删除 ExpiresAt < threshold 的记录。
	DeleteExpiredBefore(ctx context.Context, threshold time.Time) (int64, error)
	// CountCreatedSince 统计 node 在 since 之后创建的数量（用于节点级限流）。
	CountCreatedSince(ctx context.Context, nodeID uint, since time.Time) (int64, error)
}

type GormAgentInstallTokenRepository struct {
	db *gorm.DB
}

func NewAgentInstallTokenRepository(db *gorm.DB) *GormAgentInstallTokenRepository {
	return &GormAgentInstallTokenRepository{db: db}
}

func (r *GormAgentInstallTokenRepository) Create(ctx context.Context, t *model.AgentInstallToken) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *GormAgentInstallTokenRepository) FindByToken(ctx context.Context, token string) (*model.AgentInstallToken, error) {
	var item model.AgentInstallToken
	if err := r.db.WithContext(ctx).Where("token = ?", token).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// FindValidByToken 仅返回未消费且未过期的记录，过滤条件与 ConsumeByToken 对齐。
func (r *GormAgentInstallTokenRepository) FindValidByToken(ctx context.Context, token string) (*model.AgentInstallToken, error) {
	var item model.AgentInstallToken
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).
		Where("token = ? AND consumed_at IS NULL AND expires_at > ?", token, now).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// ConsumeByToken 使用条件 UPDATE + RowsAffected 实现原子消费。
// SQLite 不支持 SELECT FOR UPDATE，但 UPDATE 本身在 SQLite 中是原子的。
func (r *GormAgentInstallTokenRepository) ConsumeByToken(ctx context.Context, token string) (*model.AgentInstallToken, error) {
	var consumed *model.AgentInstallToken
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		result := tx.Model(&model.AgentInstallToken{}).
			Where("token = ? AND consumed_at IS NULL AND expires_at > ?", token, now).
			Update("consumed_at", &now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		var item model.AgentInstallToken
		if err := tx.Where("token = ?", token).First(&item).Error; err != nil {
			return err
		}
		consumed = &item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return consumed, nil
}

func (r *GormAgentInstallTokenRepository) DeleteExpiredBefore(ctx context.Context, threshold time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("expires_at < ?", threshold).Delete(&model.AgentInstallToken{})
	return result.RowsAffected, result.Error
}

func (r *GormAgentInstallTokenRepository) CountCreatedSince(ctx context.Context, nodeID uint, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.AgentInstallToken{}).
		Where("node_id = ? AND created_at >= ?", nodeID, since).
		Count(&n).Error
	return n, err
}
