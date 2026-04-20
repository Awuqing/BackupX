package repository

import (
	"context"
	"errors"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type BackupTaskListOptions struct {
	Type    string
	Enabled *bool
}

type BackupTaskRepository interface {
	List(context.Context, BackupTaskListOptions) ([]model.BackupTask, error)
	FindByID(context.Context, uint) (*model.BackupTask, error)
	FindByName(context.Context, string) (*model.BackupTask, error)
	ListSchedulable(context.Context) ([]model.BackupTask, error)
	ListVerifySchedulable(context.Context) ([]model.BackupTask, error)
	Count(context.Context) (int64, error)
	CountEnabled(context.Context) (int64, error)
	CountByStorageTargetID(context.Context, uint) (int64, error)
	CountByNodeID(context.Context, uint) (int64, error)
	ListByNodeID(context.Context, uint) ([]model.BackupTask, error)
	DistinctTags(context.Context) ([]string, error)
	Create(context.Context, *model.BackupTask) error
	Update(context.Context, *model.BackupTask) error
	Delete(context.Context, uint) error
}

type GormBackupTaskRepository struct {
	db *gorm.DB
}

func NewBackupTaskRepository(db *gorm.DB) *GormBackupTaskRepository {
	return &GormBackupTaskRepository{db: db}
}

func (r *GormBackupTaskRepository) List(ctx context.Context, options BackupTaskListOptions) ([]model.BackupTask, error) {
	query := r.db.WithContext(ctx).Model(&model.BackupTask{}).Preload("StorageTarget").Preload("StorageTargets").Preload("Node").Order("updated_at desc")
	if options.Type != "" {
		query = query.Where("type = ?", options.Type)
	}
	if options.Enabled != nil {
		query = query.Where("enabled = ?", *options.Enabled)
	}
	var items []model.BackupTask
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormBackupTaskRepository) FindByID(ctx context.Context, id uint) (*model.BackupTask, error) {
	var item model.BackupTask
	if err := r.db.WithContext(ctx).Preload("StorageTarget").Preload("StorageTargets").Preload("Node").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormBackupTaskRepository) FindByName(ctx context.Context, name string) (*model.BackupTask, error) {
	var item model.BackupTask
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormBackupTaskRepository) ListSchedulable(ctx context.Context) ([]model.BackupTask, error) {
	var items []model.BackupTask
	if err := r.db.WithContext(ctx).Preload("StorageTarget").Preload("StorageTargets").Preload("Node").Where("enabled = ? AND cron_expr <> ''", true).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListVerifySchedulable 列出所有启用且配置了验证 cron 的任务。
// 与 ListSchedulable 的区别：即使任务本身没有备份 cron，只要配置了 verify_cron_expr
// 也会被调度（验证是独立的定时动作）。
func (r *GormBackupTaskRepository) ListVerifySchedulable(ctx context.Context) ([]model.BackupTask, error) {
	var items []model.BackupTask
	if err := r.db.WithContext(ctx).
		Preload("StorageTarget").
		Preload("StorageTargets").
		Preload("Node").
		Where("enabled = ? AND verify_enabled = ? AND verify_cron_expr <> ''", true, true).
		Order("id asc").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// DistinctTags 返回系统中所有任务使用过的唯一标签（用于 UI 建议）。
// tags 字段是逗号分隔字符串，此方法会扁平化后去重。
func (r *GormBackupTaskRepository) DistinctTags(ctx context.Context) ([]string, error) {
	var rows []struct {
		Tags string
	}
	if err := r.db.WithContext(ctx).
		Model(&model.BackupTask{}).
		Select("tags").
		Where("tags <> ''").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	result := []string{}
	for _, row := range rows {
		for _, raw := range splitTags(row.Tags) {
			if !seen[raw] {
				seen[raw] = true
				result = append(result, raw)
			}
		}
	}
	return result, nil
}

// splitTags 把逗号分隔的 tags 字符串拆成 trim 后的非空切片。
func splitTags(value string) []string {
	if value == "" {
		return nil
	}
	var out []string
	for _, t := range splitAndTrim(value, ",") {
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// splitAndTrim 内部工具函数：按分隔符切分并去除每段空白。
func splitAndTrim(value, sep string) []string {
	parts := make([]string, 0)
	for _, p := range bytesSplit(value, sep) {
		trimmed := bytesTrimSpace(p)
		parts = append(parts, trimmed)
	}
	return parts
}

// bytesSplit / bytesTrimSpace 只是 strings 的薄包装，便于此仓储文件不引入 strings 依赖。
func bytesSplit(value, sep string) []string {
	out := []string{}
	start := 0
	for i := 0; i+len(sep) <= len(value); i++ {
		if value[i:i+len(sep)] == sep {
			out = append(out, value[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	out = append(out, value[start:])
	return out
}

func bytesTrimSpace(value string) string {
	start, end := 0, len(value)
	for start < end && (value[start] == ' ' || value[start] == '\t' || value[start] == '\n' || value[start] == '\r') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t' || value[end-1] == '\n' || value[end-1] == '\r') {
		end--
	}
	return value[start:end]
}

func (r *GormBackupTaskRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupTask{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormBackupTaskRepository) CountEnabled(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupTask{}).Where("enabled = ?", true).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormBackupTaskRepository) CountByStorageTargetID(ctx context.Context, storageTargetID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupTaskStorageTarget{}).Where("storage_target_id = ?", storageTargetID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountByNodeID 统计绑定到指定节点的任务数。用于删除节点前的引用检查。
func (r *GormBackupTaskRepository) CountByNodeID(ctx context.Context, nodeID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BackupTask{}).Where("node_id = ?", nodeID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// ListByNodeID 列出绑定到指定节点的任务。用于 Agent 拉取本节点待执行任务。
func (r *GormBackupTaskRepository) ListByNodeID(ctx context.Context, nodeID uint) ([]model.BackupTask, error) {
	var items []model.BackupTask
	if err := r.db.WithContext(ctx).Preload("StorageTarget").Preload("StorageTargets").Preload("Node").Where("node_id = ?", nodeID).Order("id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormBackupTaskRepository) Create(ctx context.Context, item *model.BackupTask) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return err
	}
	return r.syncStorageTargets(ctx, item)
}

func (r *GormBackupTaskRepository) Update(ctx context.Context, item *model.BackupTask) error {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return err
	}
	if len(item.StorageTargets) > 0 {
		return r.db.WithContext(ctx).Model(item).Association("StorageTargets").Replace(item.StorageTargets)
	}
	return nil
}

// syncStorageTargets 确保中间表数据一致：优先使用 StorageTargets，回退到 StorageTargetID
func (r *GormBackupTaskRepository) syncStorageTargets(ctx context.Context, item *model.BackupTask) error {
	targets := item.StorageTargets
	if len(targets) == 0 && item.StorageTargetID > 0 {
		targets = []model.StorageTarget{{ID: item.StorageTargetID}}
	}
	if len(targets) > 0 {
		return r.db.WithContext(ctx).Model(item).Association("StorageTargets").Replace(targets)
	}
	return nil
}

func (r *GormBackupTaskRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.BackupTask{}, id).Error
}
