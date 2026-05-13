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
	// CompleteDispatched 只在命令仍处于 dispatched 时写入终态。
	// 返回 false 表示命令已被超时监控或其它流程终结，调用方不应覆盖。
	CompleteDispatched(ctx context.Context, cmd *model.AgentCommand) (bool, error)
	// MarkStaleTimeout 把 dispatched 状态但超时未完成的命令标记为 timeout。
	// 返回被标记的行数。不返回具体命令（供背景监控简单调用）。
	MarkStaleTimeout(ctx context.Context, threshold time.Time) (int64, error)
	// TimeoutActive 只在命令仍处于 pending/dispatched 时写入 timeout。
	// 返回 false 表示命令已被 Agent 回写为终态，调用方不应覆盖。
	TimeoutActive(ctx context.Context, cmd *model.AgentCommand) (bool, error)
	// ListStaleDispatched 列出 dispatched 但已超时、尚未被标记的命令。
	// 调用方需要把它们逐一标记 timeout 并联动关联记录状态。
	ListStaleDispatched(ctx context.Context, threshold time.Time) ([]model.AgentCommand, error)
	// ListStaleActive 列出 pending/dispatched 但已超时、尚未完成的命令。
	// pending 使用 created_at 判定，dispatched 使用 dispatched_at 判定。
	ListStaleActive(ctx context.Context, threshold time.Time) ([]model.AgentCommand, error)
	// ListPendingByNode 列出某节点下的所有 pending/dispatched 命令。
	// 用于删除节点或节点离线时的清理。
	ListPendingByNode(ctx context.Context, nodeID uint) ([]model.AgentCommand, error)
	NodeQueueSummaries(ctx context.Context) (map[uint]AgentCommandQueueSummary, error)
}

type AgentCommandQueueSummary struct {
	NodeID         uint       `json:"nodeId"`
	Pending        int        `json:"pending"`
	Dispatched     int        `json:"dispatched"`
	Running        int        `json:"running"`
	Depth          int        `json:"depth"`
	Timeouts       int        `json:"timeouts"`
	LastError      string     `json:"lastError,omitempty"`
	OldestActiveAt *time.Time `json:"oldestActiveAt,omitempty"`
}

type agentCommandTimeoutCount struct {
	NodeID uint
	Count  int
}

type agentCommandLastError struct {
	NodeID       uint
	ErrorMessage string
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

func (r *GormAgentCommandRepository) CompleteDispatched(ctx context.Context, cmd *model.AgentCommand) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.AgentCommand{}).
		Where("id = ? AND node_id = ? AND status = ?", cmd.ID, cmd.NodeID, model.AgentCommandStatusDispatched).
		Updates(map[string]any{
			"status":        cmd.Status,
			"error_message": cmd.ErrorMessage,
			"result":        cmd.Result,
			"completed_at":  cmd.CompletedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
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

func (r *GormAgentCommandRepository) TimeoutActive(ctx context.Context, cmd *model.AgentCommand) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.AgentCommand{}).
		Where("id = ? AND status IN ?", cmd.ID, []string{model.AgentCommandStatusPending, model.AgentCommandStatusDispatched}).
		Updates(map[string]any{
			"status":        model.AgentCommandStatusTimeout,
			"error_message": cmd.ErrorMessage,
			"completed_at":  cmd.CompletedAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
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

func (r *GormAgentCommandRepository) ListStaleActive(ctx context.Context, threshold time.Time) ([]model.AgentCommand, error) {
	var items []model.AgentCommand
	if err := r.db.WithContext(ctx).
		Where(
			"(status = ? AND created_at < ?) OR (status = ? AND dispatched_at < ?)",
			model.AgentCommandStatusPending, threshold,
			model.AgentCommandStatusDispatched, threshold,
		).
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

func (r *GormAgentCommandRepository) NodeQueueSummaries(ctx context.Context) (map[uint]AgentCommandQueueSummary, error) {
	summaries, err := r.activeQueueSummaries(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.applyTerminalQueueStats(ctx, summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}

func (r *GormAgentCommandRepository) activeQueueSummaries(ctx context.Context) (map[uint]AgentCommandQueueSummary, error) {
	var items []model.AgentCommand
	if err := r.db.WithContext(ctx).
		Where("status IN ?", []string{
			model.AgentCommandStatusPending,
			model.AgentCommandStatusDispatched,
		}).
		Order("node_id asc, id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	summaries := make(map[uint]AgentCommandQueueSummary)
	for i := range items {
		cmd := &items[i]
		summary := summaries[cmd.NodeID]
		summary.NodeID = cmd.NodeID
		switch cmd.Status {
		case model.AgentCommandStatusPending:
			summary.Pending++
			summary.Depth++
			summary.OldestActiveAt = oldestTime(summary.OldestActiveAt, &cmd.CreatedAt)
		case model.AgentCommandStatusDispatched:
			summary.Dispatched++
			summary.Depth++
			if isLongRunningAgentCommand(cmd.Type) {
				summary.Running++
			}
			summary.OldestActiveAt = oldestTime(summary.OldestActiveAt, cmd.DispatchedAt)
		}
		summaries[cmd.NodeID] = summary
	}
	return summaries, nil
}

func (r *GormAgentCommandRepository) applyTerminalQueueStats(ctx context.Context, summaries map[uint]AgentCommandQueueSummary) error {
	var timeoutCounts []agentCommandTimeoutCount
	if err := r.db.WithContext(ctx).
		Model(&model.AgentCommand{}).
		Select("node_id, COUNT(*) AS count").
		Where("status = ?", model.AgentCommandStatusTimeout).
		Group("node_id").
		Scan(&timeoutCounts).Error; err != nil {
		return err
	}
	for _, item := range timeoutCounts {
		summary := summaries[item.NodeID]
		summary.NodeID = item.NodeID
		summary.Timeouts = item.Count
		summaries[item.NodeID] = summary
	}

	terminalStatuses := []string{
		model.AgentCommandStatusFailed,
		model.AgentCommandStatusTimeout,
	}
	latestByNode := r.db.WithContext(ctx).
		Model(&model.AgentCommand{}).
		Select("node_id, MAX(COALESCE(completed_at, updated_at, created_at)) AS last_error_at").
		Where("status IN ? AND error_message <> ''", terminalStatuses).
		Group("node_id")

	var lastErrors []agentCommandLastError
	if err := r.db.WithContext(ctx).
		Table("agent_commands AS cmd").
		Select("cmd.node_id, cmd.error_message").
		Joins("JOIN (?) latest ON latest.node_id = cmd.node_id AND latest.last_error_at = COALESCE(cmd.completed_at, cmd.updated_at, cmd.created_at)", latestByNode).
		Where("cmd.status IN ? AND cmd.error_message <> ''", terminalStatuses).
		Order("cmd.node_id asc, cmd.id desc").
		Scan(&lastErrors).Error; err != nil {
		return err
	}
	seenLastError := make(map[uint]struct{}, len(lastErrors))
	for _, item := range lastErrors {
		if _, ok := seenLastError[item.NodeID]; ok {
			continue
		}
		summary := summaries[item.NodeID]
		summary.NodeID = item.NodeID
		summary.LastError = item.ErrorMessage
		summaries[item.NodeID] = summary
		seenLastError[item.NodeID] = struct{}{}
	}
	return nil
}

func oldestTime(current *time.Time, candidate *time.Time) *time.Time {
	if candidate == nil {
		return current
	}
	if current == nil || candidate.Before(*current) {
		value := *candidate
		return &value
	}
	return current
}

func isLongRunningAgentCommand(commandType string) bool {
	return commandType == model.AgentCommandTypeRunTask || commandType == model.AgentCommandTypeRestoreRecord
}
