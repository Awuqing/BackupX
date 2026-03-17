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

	if err := db.AutoMigrate(&model.User{}, &model.SystemConfig{}, &model.StorageTarget{}, &model.OAuthSession{}, &model.BackupTask{}, &model.BackupRecord{}, &model.Notification{}, &model.Node{}); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	logger.Info("database initialized", zap.String("path", cfg.Path))
	return db, nil
}
