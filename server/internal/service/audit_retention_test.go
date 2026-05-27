package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

func TestAuditRetention(t *testing.T) {
	baseDir := t.TempDir()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(baseDir, "a.db")}, log)
	if err != nil {
		t.Fatal(err)
	}
	auditRepo := repository.NewAuditLogRepository(db)
	configRepo := repository.NewSystemConfigRepository(db)
	svc := NewAuditService(auditRepo)
	ctx := context.Background()

	// seed 写一条审计日志并把 created_at 强制改到 daysAgo 天前。
	seed := func(daysAgo int) {
		rec := &model.AuditLog{Username: "t", Category: "test", Action: "seed"}
		if err := auditRepo.Create(ctx, rec); err != nil {
			t.Fatalf("create: %v", err)
		}
		ts := time.Now().UTC().AddDate(0, 0, -daysAgo)
		if err := db.Model(&model.AuditLog{}).Where("id = ?", rec.ID).Update("created_at", ts).Error; err != nil {
			t.Fatalf("backdate: %v", err)
		}
	}
	count := func() int64 {
		var n int64
		db.Model(&model.AuditLog{}).Count(&n)
		return n
	}

	seed(100)
	seed(40)
	seed(0)
	if count() != 3 {
		t.Fatalf("expected 3 seeded, got %d", count())
	}

	// days<=0 不清理。
	if n, _ := svc.PurgeOlderThan(ctx, 0); n != 0 || count() != 3 {
		t.Fatalf("days<=0 must not purge (n=%d count=%d)", n, count())
	}

	// 保留 50 天 → 删除 100 天前那条。
	n, err := svc.PurgeOlderThan(ctx, 50)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 1 || count() != 2 {
		t.Fatalf("expected 1 purged / 2 remaining, got n=%d count=%d", n, count())
	}

	// 设置驱动：retention=10 天 → 再删 40 天前那条。
	if err := configRepo.Upsert(ctx, &model.SystemConfig{Key: SettingKeyAuditRetentionDays, Value: "10"}); err != nil {
		t.Fatalf("upsert setting: %v", err)
	}
	svc.runRetentionOnce(ctx, configRepo)
	if count() != 1 {
		t.Fatalf("expected 1 remaining after retention=10, got %d", count())
	}

	// retention=0 → 永久保留，不再删除。
	if err := configRepo.Upsert(ctx, &model.SystemConfig{Key: SettingKeyAuditRetentionDays, Value: "0"}); err != nil {
		t.Fatalf("upsert setting: %v", err)
	}
	svc.runRetentionOnce(ctx, configRepo)
	if count() != 1 {
		t.Fatalf("retention=0 must keep all, got %d", count())
	}
}
