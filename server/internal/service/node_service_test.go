package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func openNodeServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ns.db")),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.Node{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestBatchCreateNodes(t *testing.T) {
	db := openNodeServiceDB(t)
	svc := NewNodeService(repository.NewNodeRepository(db), "test")
	ctx := context.Background()

	items, err := svc.BatchCreate(ctx, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3, got %d", len(items))
	}
	for _, it := range items {
		if it.ID == 0 || it.Name == "" {
			t.Errorf("invalid item %+v", it)
		}
	}
}

func TestBatchCreateRejectsDuplicatesAgainstDB(t *testing.T) {
	db := openNodeServiceDB(t)
	svc := NewNodeService(repository.NewNodeRepository(db), "test")
	ctx := context.Background()

	if _, err := svc.Create(ctx, NodeCreateInput{Name: "a"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err := svc.BatchCreate(ctx, []string{"a", "b"})
	if err == nil {
		t.Fatalf("expected error on duplicate with existing")
	}
}

func TestBatchCreateRejectsIntraBatchDuplicates(t *testing.T) {
	db := openNodeServiceDB(t)
	svc := NewNodeService(repository.NewNodeRepository(db), "test")
	_, err := svc.BatchCreate(context.Background(), []string{"x", "x"})
	if err == nil {
		t.Fatalf("expected error on intra-batch duplicate")
	}
}

func TestBatchCreateLimitEnforced(t *testing.T) {
	db := openNodeServiceDB(t)
	svc := NewNodeService(repository.NewNodeRepository(db), "test")
	names := make([]string, 51)
	for i := range names {
		names[i] = "n" + string(rune('A'+i))
	}
	_, err := svc.BatchCreate(context.Background(), names)
	if err == nil {
		t.Fatalf("expected error on >50 batch")
	}
}

func TestBatchCreateSkipsEmptyLines(t *testing.T) {
	db := openNodeServiceDB(t)
	svc := NewNodeService(repository.NewNodeRepository(db), "test")
	items, err := svc.BatchCreate(context.Background(), []string{"a", "  ", "", "b"})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 (a,b), got %d", len(items))
	}
}

func TestRotateToken(t *testing.T) {
	db := openNodeServiceDB(t)
	repo := repository.NewNodeRepository(db)
	svc := NewNodeService(repo, "test")
	ctx := context.Background()

	_, err := svc.Create(ctx, NodeCreateInput{Name: "rot"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var node model.Node
	db.First(&node, "name = ?", "rot")
	oldTok := node.Token

	newTok, err := svc.RotateToken(ctx, node.ID)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if newTok == oldTok || len(newTok) != 64 {
		t.Fatalf("invalid new token: %s", newTok)
	}

	// 旧 token 仍可查（24h 内）
	found, _ := repo.FindByToken(ctx, oldTok)
	if found == nil || found.ID != node.ID {
		t.Fatalf("old token should still work via prev_token fallback")
	}
	found2, _ := repo.FindByToken(ctx, newTok)
	if found2 == nil || found2.ID != node.ID {
		t.Fatalf("new token should work")
	}

	db.First(&node, node.ID)
	if node.PrevTokenExpires == nil {
		t.Fatalf("prev_token_expires not set")
	}
	diff := node.PrevTokenExpires.Sub(time.Now().UTC())
	if diff < 23*time.Hour || diff > 25*time.Hour {
		t.Fatalf("prev_token_expires out of range: %v", diff)
	}
}

func TestRotateTokenRejectsLocal(t *testing.T) {
	db := openNodeServiceDB(t)
	repo := repository.NewNodeRepository(db)
	svc := NewNodeService(repo, "test")
	ctx := context.Background()

	if err := svc.EnsureLocalNode(ctx); err != nil {
		t.Fatalf("ensure local: %v", err)
	}
	local, _ := repo.FindLocal(ctx)
	if _, err := svc.RotateToken(ctx, local.ID); err == nil {
		t.Fatalf("expected error rotating local node")
	}
}

func TestRotateTokenNotFound(t *testing.T) {
	db := openNodeServiceDB(t)
	svc := NewNodeService(repository.NewNodeRepository(db), "test")
	if _, err := svc.RotateToken(context.Background(), 9999); err == nil {
		t.Fatalf("expected not found error")
	}
}
