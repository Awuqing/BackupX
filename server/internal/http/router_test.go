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

	"github.com/pquerna/otp/totp"
)

func TestSetupLoginAndProfileFlow(t *testing.T) {
	router, _ := newTestHTTPRouter(t)

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

func TestTrustedDeviceCookieSkipsMFA(t *testing.T) {
	router, authService := newTestHTTPRouter(t)
	if _, err := authService.Setup(context.Background(), service.SetupInput{
		Username: "admin", Password: "password-123", DisplayName: "Admin",
	}); err != nil {
		t.Fatalf("Setup error: %v", err)
	}
	totpSetup, err := authService.PrepareTwoFactor(context.Background(), "1", service.TwoFactorSetupInput{
		CurrentPassword: "password-123",
	})
	if err != nil {
		t.Fatalf("PrepareTwoFactor error: %v", err)
	}
	enableCode, err := totp.GenerateCode(totpSetup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode error: %v", err)
	}
	if _, err := authService.EnableTwoFactor(context.Background(), "1", service.EnableTwoFactorInput{Code: enableCode}); err != nil {
		t.Fatalf("EnableTwoFactor error: %v", err)
	}

	loginCode, err := totp.GenerateCode(totpSetup.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("GenerateCode login error: %v", err)
	}
	loginBody, _ := json.Marshal(map[string]any{
		"username":          "admin",
		"password":          "password-123",
		"twoFactorCode":     loginCode,
		"rememberDevice":    true,
		"trustedDeviceName": "test browser",
	})
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(loginBody))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, loginRequest)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	trustedCookie := findCookie(loginRecorder.Result().Cookies(), trustedDeviceCookieName)
	if trustedCookie == nil {
		t.Fatalf("expected trusted device cookie")
	}
	if !trustedCookie.HttpOnly {
		t.Fatalf("expected trusted device cookie to be HttpOnly")
	}
	if trustedCookie.Path != trustedDeviceCookiePath {
		t.Fatalf("expected trusted device cookie path %q, got %q", trustedDeviceCookiePath, trustedCookie.Path)
	}
	var loginResponse struct {
		Data struct {
			Token              string                       `json:"token"`
			TrustedDeviceToken string                       `json:"trustedDeviceToken"`
			TrustedDevice      *service.TrustedDeviceOutput `json:"trustedDevice"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRecorder.Body.Bytes(), &loginResponse); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if loginResponse.Data.Token == "" || loginResponse.Data.TrustedDevice == nil {
		t.Fatalf("expected login token and trusted device metadata")
	}
	if loginResponse.Data.TrustedDeviceToken != "" {
		t.Fatalf("trusted device token should not be exposed in response body")
	}

	secondBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "password-123",
	})
	secondRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBuffer(secondBody))
	secondRequest.Header.Set("Content-Type", "application/json")
	secondRequest.AddCookie(trustedCookie)
	secondRecorder := httptest.NewRecorder()
	router.ServeHTTP(secondRecorder, secondRequest)

	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("expected trusted device login 200, got %d: %s", secondRecorder.Code, secondRecorder.Body.String())
	}
}

func newTestHTTPRouter(t *testing.T) (http.Handler, *service.AuthService) {
	t.Helper()
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
	return router, authService
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
