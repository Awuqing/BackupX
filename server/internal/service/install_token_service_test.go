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

func openInstallTokenTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "it.db")),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentInstallToken{}, &model.Node{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestInstallTokenServiceCreateAndConsume(t *testing.T) {
	db := openInstallTokenTestDB(t)
	repo := repository.NewAgentInstallTokenRepository(db)
	nodeRepo := repository.NewNodeRepository(db)

	node := &model.Node{Name: "n1", Token: "agent-token"}
	if err := nodeRepo.Create(context.Background(), node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	svc := NewInstallTokenService(repo, nodeRepo)
	created, err := svc.Create(context.Background(), InstallTokenInput{
		NodeID:       node.ID,
		Mode:         model.InstallModeSystemd,
		Arch:         model.InstallArchAuto,
		AgentVersion: "v1.7.0",
		DownloadSrc:  model.InstallSourceGitHub,
		TTLSeconds:   900,
		CreatedByID:  1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Token == "" || created.ExpiresAt.Before(time.Now().UTC()) {
		t.Fatalf("invalid token: %+v", created)
	}

	consumed, err := svc.Consume(context.Background(), created.Token)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if consumed == nil || consumed.Node.ID != node.ID {
		t.Fatalf("expected consumed token for node, got %+v", consumed)
	}

	again, err := svc.Consume(context.Background(), created.Token)
	if err != nil {
		t.Fatalf("second consume err: %v", err)
	}
	if again != nil {
		t.Fatalf("expected nil on second consume")
	}
}

func TestInstallTokenServicePeekDoesNotConsume(t *testing.T) {
	db := openInstallTokenTestDB(t)
	repo := repository.NewAgentInstallTokenRepository(db)
	nodeRepo := repository.NewNodeRepository(db)
	node := &model.Node{Name: "n2", Token: "tok2"}
	_ = nodeRepo.Create(context.Background(), node)

	svc := NewInstallTokenService(repo, nodeRepo)
	out, err := svc.Create(context.Background(), InstallTokenInput{
		NodeID: node.ID, Mode: "docker", Arch: "auto",
		AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 300, CreatedByID: 1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Peek 两次都应成功（不消费）
	for i := 0; i < 2; i++ {
		rec, err := svc.Peek(context.Background(), out.Token)
		if err != nil {
			t.Fatalf("peek %d: %v", i, err)
		}
		if rec == nil || rec.Mode != "docker" {
			t.Fatalf("peek %d bad: %+v", i, rec)
		}
	}

	// 之后仍可消费
	consumed, _ := svc.Consume(context.Background(), out.Token)
	if consumed == nil {
		t.Fatalf("consume after peek failed")
	}
}

func TestInstallTokenServiceValidatesInput(t *testing.T) {
	db := openInstallTokenTestDB(t)
	nodeRepo := repository.NewNodeRepository(db)
	node := &model.Node{Name: "valid", Token: "t"}
	_ = nodeRepo.Create(context.Background(), node)

	svc := NewInstallTokenService(repository.NewAgentInstallTokenRepository(db), nodeRepo)
	cases := []struct {
		name string
		in   InstallTokenInput
	}{
		{"bad mode", InstallTokenInput{NodeID: node.ID, Mode: "xxx", Arch: "auto", AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 300, CreatedByID: 1}},
		{"bad arch", InstallTokenInput{NodeID: node.ID, Mode: "systemd", Arch: "risc", AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 300, CreatedByID: 1}},
		{"bad source", InstallTokenInput{NodeID: node.ID, Mode: "systemd", Arch: "auto", AgentVersion: "v1", DownloadSrc: "bogus", TTLSeconds: 300, CreatedByID: 1}},
		{"bad ttl low", InstallTokenInput{NodeID: node.ID, Mode: "systemd", Arch: "auto", AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 10, CreatedByID: 1}},
		{"bad ttl high", InstallTokenInput{NodeID: node.ID, Mode: "systemd", Arch: "auto", AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 999999, CreatedByID: 1}},
		{"missing version", InstallTokenInput{NodeID: node.ID, Mode: "systemd", Arch: "auto", AgentVersion: "", DownloadSrc: "github", TTLSeconds: 300, CreatedByID: 1}},
		{"missing node id", InstallTokenInput{NodeID: 0, Mode: "systemd", Arch: "auto", AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 300, CreatedByID: 1}},
		{"node not exists", InstallTokenInput{NodeID: 999, Mode: "systemd", Arch: "auto", AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 300, CreatedByID: 1}},
	}
	for _, tc := range cases {
		if _, err := svc.Create(context.Background(), tc.in); err == nil {
			t.Errorf("%s: expected validation error", tc.name)
		}
	}
}

func TestInstallTokenServiceRateLimit(t *testing.T) {
	db := openInstallTokenTestDB(t)
	nodeRepo := repository.NewNodeRepository(db)
	node := &model.Node{Name: "rl", Token: "rl"}
	_ = nodeRepo.Create(context.Background(), node)

	svc := NewInstallTokenService(repository.NewAgentInstallTokenRepository(db), nodeRepo)
	base := InstallTokenInput{
		NodeID: node.ID, Mode: "systemd", Arch: "auto",
		AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 300, CreatedByID: 1,
	}
	// 前 5 次成功
	for i := 0; i < 5; i++ {
		if _, err := svc.Create(context.Background(), base); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	// 第 6 次应被限流
	_, err := svc.Create(context.Background(), base)
	if err == nil {
		t.Fatalf("expected rate limit error")
	}
}
