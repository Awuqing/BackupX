package http

import (
	"context"
	stdhttp "net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"backupx/server/internal/installscript"
	"backupx/server/internal/model"
	"backupx/server/internal/service"
	"github.com/gin-gonic/gin"
)

// InstallHandler 公开路由（不走 JWT 中间件）：/install/:token 与 /install/:token/compose.yml。
type InstallHandler struct {
	tokenService *service.InstallTokenService
	auditService *service.AuditService
	externalURL  string
	limiter      *ipLimiter
}

// NewInstallHandler 构造 handler 并启动限流器的后台 GC 协程。
// gcCtx 控制 GC 协程生命周期，建议传入 app context。
func NewInstallHandler(gcCtx context.Context, tokenService *service.InstallTokenService, auditService *service.AuditService, externalURL string) *InstallHandler {
	limiter := newIPLimiter(20, time.Minute)
	limiter.startGC(gcCtx)
	return &InstallHandler{
		tokenService: tokenService,
		auditService: auditService,
		externalURL:  externalURL,
		limiter:      limiter,
	}
}

// Script 消费 install token 并返回 shell 脚本；Mode 由 token 存储决定（systemd/docker/foreground 均返回 shell）。
//
// 响应头策略（issue #46 教训）：
//   - Content-Type 用 text/plain 而非 text/x-shellscript：避免 Cloudflare/反向代理把
//     脚本内容按特殊类型识别并触发 minify/HTML rewrite，导致 `curl | sh` 收到非脚本内容
//   - X-Content-Type-Options: nosniff：禁止浏览器/中间层按内容嗅探改写 MIME
//   - Cache-Control: no-store：token 一次性消费，禁止任何缓存层留存旧脚本
//   - Content-Disposition: inline; filename=...：部分代理会跳过带文件名的响应
func (h *InstallHandler) Script(c *gin.Context) {
	if !h.limiter.allow(c.ClientIP()) {
		c.String(stdhttp.StatusTooManyRequests, "请求过于频繁，请稍后再试\n")
		return
	}
	token := strings.TrimSpace(c.Param("token"))
	consumed, err := h.tokenService.Consume(c.Request.Context(), token)
	if err != nil {
		c.String(stdhttp.StatusInternalServerError, "server error\n")
		return
	}
	if consumed == nil {
		c.String(stdhttp.StatusGone, "install token 不存在、已过期或已消费\n")
		return
	}
	h.recordConsumeAudit(c, consumed, "script")
	script, err := installscript.RenderScript(installscript.Context{
		MasterURL:     resolveMasterURL(c, h.externalURL),
		AgentToken:    consumed.Node.Token,
		AgentVersion:  consumed.Record.AgentVer,
		Mode:          consumed.Record.Mode,
		Arch:          consumed.Record.Arch,
		DownloadBase:  installscript.DownloadBaseFor(consumed.Record.DownloadSrc),
		InstallPrefix: "/opt/backupx-agent",
		NodeID:        consumed.Node.ID,
	})
	if err != nil {
		c.String(stdhttp.StatusInternalServerError, "render error\n")
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Disposition", `inline; filename="backupx-agent-install.sh"`)
	c.Data(stdhttp.StatusOK, "text/plain; charset=utf-8", []byte(script))
}

// Compose 消费 install token 并返回 docker-compose YAML，仅 Mode=docker 有效。
// 注意：/install/:token 与 /install/:token/compose.yml 共享同一 token 的消费状态，任一首次命中即消费。
func (h *InstallHandler) Compose(c *gin.Context) {
	if !h.limiter.allow(c.ClientIP()) {
		c.String(stdhttp.StatusTooManyRequests, "请求过于频繁，请稍后再试\n")
		return
	}
	token := strings.TrimSpace(c.Param("token"))
	// 先 Peek 看 Mode（不消费），若非 docker 直接 400
	record, err := h.tokenService.Peek(c.Request.Context(), token)
	if err != nil {
		c.String(stdhttp.StatusInternalServerError, "server error\n")
		return
	}
	if record == nil {
		c.String(stdhttp.StatusGone, "install token 不存在\n")
		return
	}
	if record.Mode != model.InstallModeDocker {
		c.String(stdhttp.StatusBadRequest, "该 install token 的模式不是 docker\n")
		return
	}
	// 消费
	consumed, err := h.tokenService.Consume(c.Request.Context(), token)
	if err != nil {
		c.String(stdhttp.StatusInternalServerError, "server error\n")
		return
	}
	if consumed == nil {
		c.String(stdhttp.StatusGone, "install token 已过期或已消费\n")
		return
	}
	h.recordConsumeAudit(c, consumed, "compose")
	yaml, err := installscript.RenderComposeYaml(installscript.Context{
		MasterURL:    resolveMasterURL(c, h.externalURL),
		AgentToken:   consumed.Node.Token,
		AgentVersion: consumed.Record.AgentVer,
		Mode:         model.InstallModeDocker,
		NodeID:       consumed.Node.ID,
	})
	if err != nil {
		c.String(stdhttp.StatusInternalServerError, "render error\n")
		return
	}
	c.Data(stdhttp.StatusOK, "text/yaml; charset=utf-8", []byte(yaml))
}

func (h *InstallHandler) recordConsumeAudit(c *gin.Context, consumed *service.ConsumedInstallToken, kind string) {
	if h.auditService == nil {
		return
	}
	h.auditService.Record(service.AuditEntry{
		Category:   "install_token",
		Action:     "consume",
		TargetType: "node",
		TargetID:   strconv.FormatUint(uint64(consumed.Node.ID), 10),
		TargetName: consumed.Node.Name,
		Detail:     "install token 消费 (" + kind + ")",
		ClientIP:   c.ClientIP(),
	})
}

// resolveMasterURL 按优先级推导 Master URL：外部配置 > X-Forwarded-* > Request.Host。
// 此为包级 helper，供 install_handler 和 node_handler 共用。
func resolveMasterURL(c *gin.Context, externalURL string) string {
	if strings.TrimSpace(externalURL) != "" {
		return strings.TrimRight(externalURL, "/")
	}
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host
}

// ipLimiter 简单内存滑动窗口限流，按 client IP 维度。
type ipLimiter struct {
	mu     sync.Mutex
	events map[string][]time.Time
	limit  int
	window time.Duration
}

func newIPLimiter(limit int, window time.Duration) *ipLimiter {
	return &ipLimiter{events: make(map[string][]time.Time), limit: limit, window: window}
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	keep := l.events[ip][:0]
	for _, t := range l.events[ip] {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	if len(keep) >= l.limit {
		l.events[ip] = keep
		return false
	}
	l.events[ip] = append(keep, now)
	return true
}

// gc 清理窗口外所有过期的 IP 条目，防止公网扫描导致 map 无界增长。
// 由后台 goroutine 周期性调用。
func (l *ipLimiter) gc(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := now.Add(-l.window)
	for k, v := range l.events {
		stale := true
		for _, t := range v {
			if t.After(cutoff) {
				stale = false
				break
			}
		}
		if stale {
			delete(l.events, k)
		}
	}
}

// startGC 启动后台清理协程，每 window 周期清扫一次 map。
// ctx 取消时协程退出。
func (l *ipLimiter) startGC(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(l.window)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				l.gc(t)
			}
		}
	}()
}
