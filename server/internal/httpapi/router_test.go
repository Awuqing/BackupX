//go:build ignore

package httpapi

import (
	"bytes"
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
)

func TestSetupLoginProfileAndSystemInfo(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		Server:   config.ServerConfig{Mode: "test"},
		Database: config.DatabaseConfig{Path: filepath.Join(tmpDir, "backupx.db")},
		Security: config.SecurityConfig{JWTSecret: "test-jwt-secret", JWTExpire: "1h", EncryptionKey: "test-encryption-key"},
		Log:      config.LogConfig{Level: "error"},
	}
	log, err := logger.New(cfg.Log)
	if err != nil {
		t.Fatalf("logger.New() error = %v", err)
	}
	db, err := database.Open(cfg.Database, log)
	if err != nil {
		t.Fatalf("database.Open() error = %v", err)
	}
	jwtManager := security.NewJWTManager(cfg.Security.JWTSecret, time.Hour)
	authService := service.NewAuthService(repository.NewUserRepository(db), jwtManager, security.NewLoginLimiter(5, time.Minute))
	systemService := service.NewSystemService(cfg, "test", time.Now().Add(-time.Minute))
	router := NewRouter(Dependencies{Logger: log, AuthService: authService, SystemService: systemService, JWTManager: jwtManager, Mode: "test"})

	setupBody := map[string]string{"username": "admin", "password": "super-secret", "displayName": "管理员"}
	setupResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/setup", setupBody, "")
	if setupResp.Code != http.StatusCreated {
		t.Fatalf("unexpected setup status: %d body=%s", setupResp.Code, setupResp.Body.String())
	}
	var setupPayload struct {
		Code string `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(setupResp.Body.Bytes(), &setupPayload); err != nil {
		t.Fatalf("decode setup response: %v", err)
	}
	if setupPayload.Data.Token == "" {
		t.Fatal("expected token in setup response")
	}

	profileResp := performJSONRequest(t, router, http.MethodGet, "/api/auth/profile", nil, setupPayload.Data.Token)
	if profileResp.Code != http.StatusOK {
		t.Fatalf("unexpected profile status: %d body=%s", profileResp.Code, profileResp.Body.String())
	}

	loginBody := map[string]string{"username": "admin", "password": "super-secret"}
	loginResp := performJSONRequest(t, router, http.MethodPost, "/api/auth/login", loginBody, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("unexpected login status: %d body=%s", loginResp.Code, loginResp.Body.String())
	}

	systemResp := performJSONRequest(t, router, http.MethodGet, "/api/system/info", nil, setupPayload.Data.Token)
	if systemResp.Code != http.StatusOK {
		t.Fatalf("unexpected system info status: %d body=%s", systemResp.Code, systemResp.Body.String())
	}
}

func performJSONRequest(t *testing.T, handler http.Handler, method string, path string, payload any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var body []byte
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		body = encoded
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
