package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"backupx/server/internal/config"
	"backupx/server/internal/model"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func Open(cfg config.DatabaseConfig, logger *zap.Logger) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}

	separator := "?"
	if strings.Contains(cfg.Path, "?") {
		separator = "&"
	}
	// busy_timeout 减少 Agent 轮询、心跳和任务写入同时发生时的瞬时锁错误。
	// 维持默认回滚日志模式，保证当前嵌入式 SQLite 依赖的数据完整性。
	dsn := cfg.Path + separator + "_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.AutoMigrate(&model.User{}, &model.SystemConfig{}, &model.StorageTarget{}, &model.OAuthSession{}, &model.BackupTask{}, &model.BackupRecord{}, &model.Notification{}, &model.Node{}, &model.BackupTaskStorageTarget{}, &model.AuditLog{}, &model.AgentCommand{}, &model.AgentInstallToken{}, &model.RestoreRecord{}, &model.VerificationRecord{}, &model.ApiKey{}, &model.ReplicationRecord{}, &model.TaskTemplate{}); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	// 一次性数据迁移：从 backup_tasks.storage_target_id 回填到多对多中间表
	var count int64
	db.Model(&model.BackupTaskStorageTarget{}).Count(&count)
	if count == 0 {
		db.Exec("INSERT INTO backup_task_storage_targets (backup_task_id, storage_target_id) SELECT id, storage_target_id FROM backup_tasks WHERE storage_target_id > 0")
	}

	logger.Info("database initialized", zap.String("path", cfg.Path))
	return db, nil
}
