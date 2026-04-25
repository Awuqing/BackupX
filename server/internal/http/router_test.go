package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
	"backupx/server/internal/service"
	"backupx/server/internal/storage/codec"
)

func TestSetupLoginAndProfileFlow(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.Config{
		Server:   config.ServerConfig{Host: "127.0.0.1", Port: 8340, Mode: "test"},
		Database: config.DatabaseConfig{Path: filepath.Join(tempDir, "backupx.db")},
		Security: config.SecurityConfig{JWTExpire: "24h"},
		Log:      config.LogConfig{Level: "error"},
	}

	log, err := logger.New(cfg.Log)
	if err != nil {
		t.Fatalf("logger.New error: %v", err)
	}
	db, err := database.Open(cfg.Database, log)
	if err != nil {
		t.Fatalf("database.Open error: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB error: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	userRepo := repository.NewUserRepository(db)
	systemConfigRepo := repository.NewSystemConfigRepository(db)
	resolved, err := service.ResolveSecurity(context.Background(), cfg.Security, systemConfigRepo)
	if err != nil {
		t.Fatalf("ResolveSecurity error: %v", err)
	}
	jwtManager := security.NewJWTManager(resolved.JWTSecret, time.Hour)
	authService := service.NewAuthService(userRepo, systemConfigRepo, jwtManager, security.NewLoginRateLimiter(5, time.Minute), codec.NewConfigCipher(resolved.EncryptionKey))
	systemService := service.NewSystemService(cfg, "test", time.Now().UTC())

	router := NewRouter(RouterDependencies{
		Config:           cfg,
		Version:          "test",
		Logger:           log,
		AuthService:      authService,
		SystemService:    systemService,
		JWTManager:       jwtManager,
		UserRepository:   userRepo,
		SystemConfigRepo: systemConfigRepo,
	})

	setupBody, _ := json.Marshal(map[string]string{
		"username":    "admin",
		"password":    "password-123",
		"displayName": "Admin",
	})
	setupRequest := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewBuffer(setupBody))
	setupRequest.Header.Set("Content-Type", "application/json")
	setupRecorder := httptest.NewRecorder()
	router.ServeHTTP(setupRecorder, setupRequest)

	if setupRecorder.Code != http.StatusOK {
		t.Fatalf("expected setup 200, got %d", setupRecorder.Code)
	}

	var setupResponse struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(setupRecorder.Body.Bytes(), &setupResponse); err != nil {
		t.Fatalf("unmarshal setup response: %v", err)
	}
	if setupResponse.Data.Token == "" {
		t.Fatalf("expected token in setup response")
	}

	profileRequest := httptest.NewRequest(http.MethodGet, "/api/auth/profile", nil)
	profileRequest.Header.Set("Authorization", "Bearer "+setupResponse.Data.Token)
	profileRecorder := httptest.NewRecorder()
	router.ServeHTTP(profileRecorder, profileRequest)

	if profileRecorder.Code != http.StatusOK {
		t.Fatalf("expected profile 200, got %d", profileRecorder.Code)
	}
}
