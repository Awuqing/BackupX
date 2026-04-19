package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"backupx/server/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func openTestInstallTokenDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "install.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentInstallToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestInstallTokenConsumeOnce(t *testing.T) {
	db := openTestInstallTokenDB(t)
	repo := NewAgentInstallTokenRepository(db)
	ctx := context.Background()

	tok := &model.AgentInstallToken{
		Token: "abc", NodeID: 1, Mode: model.InstallModeSystemd,
		Arch: model.InstallArchAuto, AgentVer: "v1.7.0",
		DownloadSrc: model.InstallSourceGitHub,
		ExpiresAt:   time.Now().UTC().Add(15 * time.Minute),
		CreatedByID: 1,
	}
	if err := repo.Create(ctx, tok); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.ConsumeByToken(ctx, "abc")
	if err != nil {
		t.Fatalf("consume err: %v", err)
	}
	if got == nil || got.ConsumedAt == nil {
		t.Fatalf("expected consumed token, got %+v", got)
	}

	got, err = repo.ConsumeByToken(ctx, "abc")
	if err != nil {
		t.Fatalf("second consume err: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil on second consume, got %+v", got)
	}
}

func TestInstallTokenConsumeExpired(t *testing.T) {
	db := openTestInstallTokenDB(t)
	repo := NewAgentInstallTokenRepository(db)
	ctx := context.Background()

	tok := &model.AgentInstallToken{
		Token: "stale", NodeID: 1, Mode: model.InstallModeSystemd,
		Arch: model.InstallArchAuto, AgentVer: "v1.7.0",
		DownloadSrc: model.InstallSourceGitHub,
		ExpiresAt:   time.Now().UTC().Add(-time.Minute),
		CreatedByID: 1,
	}
	if err := repo.Create(ctx, tok); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.ConsumeByToken(ctx, "stale")
	if err != nil {
		t.Fatalf("consume err: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil on expired, got %+v", got)
	}
}

func TestInstallTokenGC(t *testing.T) {
	db := openTestInstallTokenDB(t)
	repo := NewAgentInstallTokenRepository(db)
	ctx := context.Background()

	old := &model.AgentInstallToken{
		Token: "old", NodeID: 1, Mode: model.InstallModeSystemd,
		Arch: model.InstallArchAuto, AgentVer: "v1.7.0",
		DownloadSrc: model.InstallSourceGitHub,
		ExpiresAt:   time.Now().UTC().Add(-8 * 24 * time.Hour),
		CreatedByID: 1,
	}
	if err := repo.Create(ctx, old); err != nil {
		t.Fatalf("create old: %v", err)
	}

	fresh := &model.AgentInstallToken{
		Token: "fresh", NodeID: 1, Mode: model.InstallModeSystemd,
		Arch: model.InstallArchAuto, AgentVer: "v1.7.0",
		DownloadSrc: model.InstallSourceGitHub,
		ExpiresAt:   time.Now().UTC().Add(-1 * time.Hour),
		CreatedByID: 1,
	}
	if err := repo.Create(ctx, fresh); err != nil {
		t.Fatalf("create fresh: %v", err)
	}

	n, err := repo.DeleteExpiredBefore(ctx, time.Now().UTC().Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("gc err: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted, got %d", n)
	}
}

func TestInstallTokenCountCreatedSince(t *testing.T) {
	db := openTestInstallTokenDB(t)
	repo := NewAgentInstallTokenRepository(db)
	ctx := context.Background()

	// 同一节点 3 条
	for i := 0; i < 3; i++ {
		_ = repo.Create(ctx, &model.AgentInstallToken{
			Token: "t" + string(rune('a'+i)), NodeID: 1, Mode: "systemd", Arch: "auto",
			AgentVer: "v1", DownloadSrc: "github",
			ExpiresAt: time.Now().UTC().Add(time.Minute), CreatedByID: 1,
		})
	}
	// 另一节点 2 条（不计入）
	for i := 0; i < 2; i++ {
		_ = repo.Create(ctx, &model.AgentInstallToken{
			Token: "n2_" + string(rune('a'+i)), NodeID: 2, Mode: "systemd", Arch: "auto",
			AgentVer: "v1", DownloadSrc: "github",
			ExpiresAt: time.Now().UTC().Add(time.Minute), CreatedByID: 1,
		})
	}

	n, err := repo.CountCreatedSince(ctx, 1, time.Now().UTC().Add(-time.Minute))
	if err != nil {
		t.Fatalf("count err: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3, got %d", n)
	}
}
