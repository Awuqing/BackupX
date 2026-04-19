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

func openTestNodeDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nodes.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Node{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestFindByTokenFallsBackToPrevToken(t *testing.T) {
	db := openTestNodeDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	future := time.Now().UTC().Add(24 * time.Hour)
	node := &model.Node{
		Name: "test", Token: "new-token",
		PrevToken: "old-token", PrevTokenExpires: &future,
	}
	if err := repo.Create(ctx, node); err != nil {
		t.Fatalf("create: %v", err)
	}

	// 新 token 能查到
	got, err := repo.FindByToken(ctx, "new-token")
	if err != nil || got == nil || got.ID != node.ID {
		t.Fatalf("new token lookup failed: err=%v got=%v", err, got)
	}

	// 旧 token 也能查到（未过期）
	got, err = repo.FindByToken(ctx, "old-token")
	if err != nil || got == nil || got.ID != node.ID {
		t.Fatalf("prev_token lookup failed: err=%v got=%v", err, got)
	}
}

func TestFindByTokenRejectsExpiredPrevToken(t *testing.T) {
	db := openTestNodeDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	past := time.Now().UTC().Add(-1 * time.Hour)
	node := &model.Node{
		Name: "test", Token: "new-token",
		PrevToken: "stale", PrevTokenExpires: &past,
	}
	if err := repo.Create(ctx, node); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.FindByToken(ctx, "stale")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != nil {
		t.Fatalf("expected stale prev_token rejected, got %v", got)
	}
}
