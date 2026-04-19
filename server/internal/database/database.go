package database

import (
	"fmt"
	"os"
	"path/filepath"

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

	db, err := gorm.Open(sqlite.Open(cfg.Path), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.AutoMigrate(&model.User{}, &model.SystemConfig{}, &model.StorageTarget{}, &model.OAuthSession{}, &model.BackupTask{}, &model.BackupRecord{}, &model.Notification{}, &model.Node{}, &model.BackupTaskStorageTarget{}, &model.AuditLog{}, &model.AgentCommand{}, &model.AgentInstallToken{}); err != nil {
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
