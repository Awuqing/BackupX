package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
	"backupx/server/internal/service"
)

// setupInstallFlowRouter 构造一个 Node + Agent + InstallToken 全量依赖的 router，
// 并返回已登录管理员 JWT。
func setupInstallFlowRouter(t *testing.T) (http.Handler, string) {
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
		t.Fatalf("logger: %v", err)
	}
	db, err := database.Open(cfg.Database, log)
	if err != nil {
		t.Fatalf("db: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	systemConfigRepo := repository.NewSystemConfigRepository(db)
	resolved, err := service.ResolveSecurity(context.Background(), cfg.Security, systemConfigRepo)
	if err != nil {
		t.Fatalf("security: %v", err)
	}
	jwtMgr := security.NewJWTManager(resolved.JWTSecret, time.Hour)
	authSvc := service.NewAuthService(userRepo, systemConfigRepo, jwtMgr, security.NewLoginRateLimiter(5, time.Minute))
	systemSvc := service.NewSystemService(cfg, "test", time.Now().UTC())

	nodeRepo := repository.NewNodeRepository(db)
	nodeSvc := service.NewNodeService(nodeRepo, "test")
	if err := nodeSvc.EnsureLocalNode(context.Background()); err != nil {
		t.Fatalf("ensure local: %v", err)
	}

	installTokenRepo := repository.NewAgentInstallTokenRepository(db)
	installTokenSvc := service.NewInstallTokenService(installTokenRepo, nodeRepo)

	auditLogRepo := repository.NewAuditLogRepository(db)
	auditSvc := service.NewAuditService(auditLogRepo)

	// 用 cancelable ctx，测试结束时停掉 handler 启动的后台 GC 协程，
	// 避免 goroutine 持有 map 导致 tempdir 清理失败。
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	router := NewRouter(RouterDependencies{
		Context:             ctx,
		Config:              cfg,
		Version:             "test",
		Logger:              log,
		AuthService:         authSvc,
		SystemService:       systemSvc,
		NodeService:         nodeSvc,
		InstallTokenService: installTokenSvc,
		AuditService:        auditSvc,
		JWTManager:          jwtMgr,
		UserRepository:      userRepo,
		SystemConfigRepo:    systemConfigRepo,
	})

	// setup 管理员并登录拿 JWT
	setupBody, _ := json.Marshal(map[string]string{
		"username": "admin", "password": "password-123", "displayName": "admin",
	})
	setupReq := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewBuffer(setupBody))
	setupReq.Header.Set("Content-Type", "application/json")
	setupRec := httptest.NewRecorder()
	router.ServeHTTP(setupRec, setupReq)
	if setupRec.Code != 200 {
		t.Fatalf("setup failed: %d %s", setupRec.Code, setupRec.Body.String())
	}
	var setupResp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(setupRec.Body.Bytes(), &setupResp); err != nil {
		t.Fatalf("unmarshal setup: %v", err)
	}

	return router, setupResp.Data.Token
}

func TestOneClickInstallFlow(t *testing.T) {
	router, jwt := setupInstallFlowRouter(t)

	// 1. 批量创建
	batchBody, _ := json.Marshal(map[string][]string{"names": {"prod-a", "prod-b"}})
	batchReq := httptest.NewRequest(http.MethodPost, "/api/nodes/batch", bytes.NewBuffer(batchBody))
	batchReq.Header.Set("Content-Type", "application/json")
	batchReq.Header.Set("Authorization", "Bearer "+jwt)
	batchRec := httptest.NewRecorder()
	router.ServeHTTP(batchRec, batchReq)
	if batchRec.Code != 200 {
		t.Fatalf("batch create failed: %d %s", batchRec.Code, batchRec.Body.String())
	}
	var batchResp struct {
		Data []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(batchRec.Body.Bytes(), &batchResp); err != nil {
		t.Fatalf("unmarshal batch: %v", err)
	}
	if len(batchResp.Data) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(batchResp.Data))
	}
	nodeID := batchResp.Data[0].ID

	// 2. 生成 install token
	genBody, _ := json.Marshal(map[string]any{
		"mode":         "systemd",
		"arch":         "auto",
		"agentVersion": "v1.7.0",
		"downloadSrc":  "github",
		"ttlSeconds":   900,
	})
	genReq := httptest.NewRequest(http.MethodPost,
		"/api/nodes/"+formatUint(nodeID)+"/install-tokens", bytes.NewBuffer(genBody))
	genReq.Header.Set("Content-Type", "application/json")
	genReq.Header.Set("Authorization", "Bearer "+jwt)
	genRec := httptest.NewRecorder()
	router.ServeHTTP(genRec, genReq)
	if genRec.Code != 200 {
		t.Fatalf("install-tokens failed: %d %s", genRec.Code, genRec.Body.String())
	}
	var genResp struct {
		Data struct {
			InstallToken string `json:"installToken"`
			URL          string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(genRec.Body.Bytes(), &genResp); err != nil {
		t.Fatalf("unmarshal gen: %v", err)
	}
	if genResp.Data.InstallToken == "" {
		t.Fatalf("missing installToken")
	}

	// 3. 公开端点消费
	scriptReq := httptest.NewRequest(http.MethodGet, "/install/"+genResp.Data.InstallToken, nil)
	scriptRec := httptest.NewRecorder()
	router.ServeHTTP(scriptRec, scriptReq)
	if scriptRec.Code != 200 {
		t.Fatalf("script fetch failed: %d %s", scriptRec.Code, scriptRec.Body.String())
	}
	if !strings.Contains(scriptRec.Body.String(), "systemctl enable --now backupx-agent") {
		t.Fatalf("script missing systemctl enable:\n%s", scriptRec.Body.String())
	}
	// Issue #46 防嗅探 headers：text/plain + nosniff + no-store + Content-Disposition
	if ct := scriptRec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("script Content-Type should be text/plain*, got %q", ct)
	}
	if nosniff := scriptRec.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Errorf("missing X-Content-Type-Options: nosniff (got %q)", nosniff)
	}
	if cc := scriptRec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("missing Cache-Control: no-store (got %q)", cc)
	}
	if cd := scriptRec.Header().Get("Content-Disposition"); !strings.Contains(cd, "backupx-agent-install.sh") {
		t.Errorf("Content-Disposition should name the script file (got %q)", cd)
	}
	if !strings.Contains(scriptRec.Body.String(), "BACKUPX_AGENT_INSTALL_V1") {
		t.Errorf("script missing magic marker BACKUPX_AGENT_INSTALL_V1")
	}

	// 4. 再次消费应 410
	scriptReq2 := httptest.NewRequest(http.MethodGet, "/install/"+genResp.Data.InstallToken, nil)
	scriptRec2 := httptest.NewRecorder()
	router.ServeHTTP(scriptRec2, scriptReq2)
	if scriptRec2.Code != http.StatusGone {
		t.Fatalf("second consume should be 410, got %d: %s", scriptRec2.Code, scriptRec2.Body.String())
	}
}

// TestInstallScriptAliasUnderAPI 验证 /api/install/:token 别名路径可用，
// 这是 Issue #46 的根本修复：让 install 端点自动命中反向代理的 /api/ 转发规则，
// 避免 nginx SPA fallback 把请求当前端路由返回 index.html。
func TestInstallScriptAliasUnderAPI(t *testing.T) {
	router, token := setupInstallFlowRouter(t)

	// 1. 创建一个节点，生成 install token
	batchBody, _ := json.Marshal(map[string][]string{"names": {"alias-node"}})
	batchReq := httptest.NewRequest(http.MethodPost, "/api/nodes/batch", bytes.NewReader(batchBody))
	batchReq.Header.Set("Content-Type", "application/json")
	batchReq.Header.Set("Authorization", "Bearer "+token)
	batchRec := httptest.NewRecorder()
	router.ServeHTTP(batchRec, batchReq)
	if batchRec.Code != 200 {
		t.Fatalf("batch create failed: %d %s", batchRec.Code, batchRec.Body.String())
	}
	var batchResp struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(batchRec.Body.Bytes(), &batchResp)
	if len(batchResp.Data) == 0 {
		t.Fatalf("batch create returned no nodes: %s", batchRec.Body.String())
	}
	nodeID := batchResp.Data[0].ID

	genBody, _ := json.Marshal(map[string]any{
		"mode": "systemd", "arch": "auto", "agentVersion": "v1.7.0", "downloadSrc": "github", "ttlSeconds": 600,
	})
	genReq := httptest.NewRequest(http.MethodPost,
		"/api/nodes/"+strconv.FormatUint(uint64(nodeID), 10)+"/install-tokens", bytes.NewReader(genBody))
	genReq.Header.Set("Content-Type", "application/json")
	genReq.Header.Set("Authorization", "Bearer "+token)
	genRec := httptest.NewRecorder()
	router.ServeHTTP(genRec, genReq)
	if genRec.Code != 200 {
		t.Fatalf("gen install token failed: %d %s", genRec.Code, genRec.Body.String())
	}
	var genResp struct {
		Data struct {
			InstallToken string `json:"installToken"`
			URL          string `json:"url"`
		} `json:"data"`
	}
	_ = json.Unmarshal(genRec.Body.Bytes(), &genResp)

	// 2. 新生成的 url 应指向 /api/install/... —— 让反向代理的 /api/ 转发规则自动接管
	if !strings.Contains(genResp.Data.URL, "/api/install/") {
		t.Errorf("new install URL should use /api/install/ prefix, got %s", genResp.Data.URL)
	}

	// 3. /api/install/:token 必须可消费（与 /install/:token 等价）
	aliasReq := httptest.NewRequest(http.MethodGet, "/api/install/"+genResp.Data.InstallToken, nil)
	aliasRec := httptest.NewRecorder()
	router.ServeHTTP(aliasRec, aliasReq)
	if aliasRec.Code != 200 {
		t.Fatalf("/api/install alias failed: %d %s", aliasRec.Code, aliasRec.Body.String())
	}
	if !strings.Contains(aliasRec.Body.String(), "systemctl enable --now backupx-agent") {
		t.Errorf("alias should return rendered script, got:\n%s", aliasRec.Body.String())
	}
	if ct := aliasRec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("alias Content-Type should be text/plain*, got %q", ct)
	}
}

func TestInstallTokenRateLimit(t *testing.T) {
	router, jwt := setupInstallFlowRouter(t)

	batchBody, _ := json.Marshal(map[string][]string{"names": {"rl-test"}})
	batchReq := httptest.NewRequest(http.MethodPost, "/api/nodes/batch", bytes.NewBuffer(batchBody))
	batchReq.Header.Set("Content-Type", "application/json")
	batchReq.Header.Set("Authorization", "Bearer "+jwt)
	batchRec := httptest.NewRecorder()
	router.ServeHTTP(batchRec, batchReq)
	if batchRec.Code != 200 {
		t.Fatalf("batch: %d %s", batchRec.Code, batchRec.Body.String())
	}
	var batchResp struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(batchRec.Body.Bytes(), &batchResp)
	nodeID := batchResp.Data[0].ID

	body, _ := json.Marshal(map[string]any{
		"mode": "systemd", "arch": "auto", "agentVersion": "v1",
		"downloadSrc": "github", "ttlSeconds": 300,
	})
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost,
			"/api/nodes/"+formatUint(nodeID)+"/install-tokens", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+jwt)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("iter %d expected 200, got %d: %s", i, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost,
		"/api/nodes/"+formatUint(nodeID)+"/install-tokens", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRotateTokenFlow(t *testing.T) {
	router, jwt := setupInstallFlowRouter(t)

	batchBody, _ := json.Marshal(map[string][]string{"names": {"rot-x"}})
	batchReq := httptest.NewRequest(http.MethodPost, "/api/nodes/batch", bytes.NewBuffer(batchBody))
	batchReq.Header.Set("Content-Type", "application/json")
	batchReq.Header.Set("Authorization", "Bearer "+jwt)
	batchRec := httptest.NewRecorder()
	router.ServeHTTP(batchRec, batchReq)
	var batchResp struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(batchRec.Body.Bytes(), &batchResp)
	nodeID := batchResp.Data[0].ID

	rotReq := httptest.NewRequest(http.MethodPost,
		"/api/nodes/"+formatUint(nodeID)+"/rotate-token", nil)
	rotReq.Header.Set("Authorization", "Bearer "+jwt)
	rotRec := httptest.NewRecorder()
	router.ServeHTTP(rotRec, rotReq)
	if rotRec.Code != 200 {
		t.Fatalf("rotate failed: %d %s", rotRec.Code, rotRec.Body.String())
	}
	var rotResp struct {
		Data struct {
			NewToken string `json:"newToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rotRec.Body.Bytes(), &rotResp)
	if len(rotResp.Data.NewToken) != 64 {
		t.Fatalf("new token wrong length: %s", rotResp.Data.NewToken)
	}
}

func TestInstallFlowComposeModeMismatch(t *testing.T) {
	router, jwt := setupInstallFlowRouter(t)

	batchBody, _ := json.Marshal(map[string][]string{"names": {"cm"}})
	batchReq := httptest.NewRequest(http.MethodPost, "/api/nodes/batch", bytes.NewBuffer(batchBody))
	batchReq.Header.Set("Content-Type", "application/json")
	batchReq.Header.Set("Authorization", "Bearer "+jwt)
	batchRec := httptest.NewRecorder()
	router.ServeHTTP(batchRec, batchReq)
	var batchResp struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(batchRec.Body.Bytes(), &batchResp)
	nodeID := batchResp.Data[0].ID

	// 生成 systemd 模式的 token
	genBody, _ := json.Marshal(map[string]any{
		"mode": "systemd", "arch": "auto", "agentVersion": "v1",
		"downloadSrc": "github", "ttlSeconds": 300,
	})
	genReq := httptest.NewRequest(http.MethodPost,
		"/api/nodes/"+formatUint(nodeID)+"/install-tokens", bytes.NewBuffer(genBody))
	genReq.Header.Set("Content-Type", "application/json")
	genReq.Header.Set("Authorization", "Bearer "+jwt)
	genRec := httptest.NewRecorder()
	router.ServeHTTP(genRec, genReq)
	var genResp struct {
		Data struct {
			InstallToken string `json:"installToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(genRec.Body.Bytes(), &genResp)

	// 访问 compose.yml 应 400
	req := httptest.NewRequest(http.MethodGet,
		"/install/"+genResp.Data.InstallToken+"/compose.yml", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for mode mismatch, got %d: %s", rec.Code, rec.Body.String())
	}
	// systemd token 未被消费（Peek 不消费）→ 应仍可通过 /install/:token 消费成功
	req2 := httptest.NewRequest(http.MethodGet, "/install/"+genResp.Data.InstallToken, nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("original script fetch should still work: %d %s", rec2.Code, rec2.Body.String())
	}
}

// formatUint 小工具：uint → 十进制字符串（无需引入 strconv）。
func formatUint(u uint) string {
	if u == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	return string(buf[i:])
}
