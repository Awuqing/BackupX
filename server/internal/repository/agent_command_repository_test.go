package repository

import (
	"context"
	"testing"
	"time"

	"backupx/server/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentCommand{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAgentCommandRepository_ClaimPending(t *testing.T) {
	db := newTestDB(t)
	repo := NewAgentCommandRepository(db)
	ctx := context.Background()

	// 插入两条 pending 命令
	c1 := &model.AgentCommand{NodeID: 5, Type: "run_task", Status: model.AgentCommandStatusPending, Payload: `{"taskId":1}`}
	c2 := &model.AgentCommand{NodeID: 5, Type: "list_dir", Status: model.AgentCommandStatusPending, Payload: `{"path":"/"}`}
	c3 := &model.AgentCommand{NodeID: 99, Type: "run_task", Status: model.AgentCommandStatusPending}
	for _, c := range []*model.AgentCommand{c1, c2, c3} {
		if err := repo.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	// 第一次 Claim 应拿到 c1
	claimed, err := repo.ClaimPending(ctx, 5)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed == nil || claimed.ID != c1.ID || claimed.Status != model.AgentCommandStatusDispatched {
		t.Fatalf("expected c1 dispatched: %+v", claimed)
	}

	// 第二次应拿到 c2
	claimed2, err := repo.ClaimPending(ctx, 5)
	if err != nil || claimed2 == nil || claimed2.ID != c2.ID {
		t.Fatalf("expected c2: %+v %v", claimed2, err)
	}

	// 第三次无 pending，返回 nil
	claimed3, err := repo.ClaimPending(ctx, 5)
	if err != nil || claimed3 != nil {
		t.Fatalf("expected nil, got %+v", claimed3)
	}

	// 不同 node 的命令不应被抢到
	other, err := repo.ClaimPending(ctx, 5)
	if err != nil || other != nil {
		t.Fatalf("expected nil: %+v", other)
	}
}

func TestAgentCommandRepository_Update(t *testing.T) {
	db := newTestDB(t)
	repo := NewAgentCommandRepository(db)
	ctx := context.Background()
	cmd := &model.AgentCommand{NodeID: 1, Type: "run_task", Status: model.AgentCommandStatusPending}
	_ = repo.Create(ctx, cmd)

	cmd.Status = model.AgentCommandStatusSucceeded
	cmd.Result = `{"ok":true}`
	now := time.Now().UTC()
	cmd.CompletedAt = &now
	if err := repo.Update(ctx, cmd); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByID(ctx, cmd.ID)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if got.Status != model.AgentCommandStatusSucceeded || got.Result != `{"ok":true}` {
		t.Errorf("mismatch: %+v", got)
	}
}

func TestAgentCommandRepository_MarkStaleTimeout(t *testing.T) {
	db := newTestDB(t)
	repo := NewAgentCommandRepository(db)
	ctx := context.Background()
	old := time.Now().Add(-time.Hour)
	recent := time.Now()
	// 两条 dispatched：一条旧、一条新
	oldCmd := &model.AgentCommand{NodeID: 1, Type: "run_task", Status: model.AgentCommandStatusDispatched, DispatchedAt: &old}
	newCmd := &model.AgentCommand{NodeID: 1, Type: "run_task", Status: model.AgentCommandStatusDispatched, DispatchedAt: &recent}
	_ = repo.Create(ctx, oldCmd)
	_ = repo.Create(ctx, newCmd)

	n, err := repo.MarkStaleTimeout(ctx, time.Now().Add(-30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 row, got %d", n)
	}
	oldGot, _ := repo.FindByID(ctx, oldCmd.ID)
	newGot, _ := repo.FindByID(ctx, newCmd.ID)
	if oldGot.Status != model.AgentCommandStatusTimeout {
		t.Errorf("old should be timeout: %+v", oldGot)
	}
	if newGot.Status != model.AgentCommandStatusDispatched {
		t.Errorf("new should stay dispatched: %+v", newGot)
	}
}
