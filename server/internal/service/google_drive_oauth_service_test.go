package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
)

func TestGoogleDriveOAuthServiceStartAndComplete(t *testing.T) {
	tempDir := t.TempDir()
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(tempDir, "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	sessions := repository.NewOAuthSessionRepository(db)
	service := NewGoogleDriveOAuthService(sessions, codec.New("encryption-secret"))
	service.now = func() time.Time { return time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC) }
	service.generateState = func() (string, error) { return "oauth-state", nil }
	service.exchangeCode = func(context.Context, *oauth2.Config, string) (*oauth2.Token, error) {
		return &oauth2.Token{RefreshToken: "refresh-token"}, nil
	}

	url, state, err := service.Start(context.Background(), nil, storage.GoogleDriveConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost:8340/api/storage-targets/google-drive/callback",
		FolderID:     "folder-id",
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if state != "oauth-state" {
		t.Fatalf("expected deterministic state, got %s", state)
	}
	if !strings.Contains(url, "oauth-state") {
		t.Fatalf("expected auth url to contain state, got %s", url)
	}

	result, err := service.Complete(context.Background(), state, "auth-code")
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if result.Config.RefreshToken != "refresh-token" {
		t.Fatalf("expected refresh token to be persisted")
	}
}
