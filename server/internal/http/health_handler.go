package http

import (
	stdhttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// HealthHandler 提供 K8s/Swarm 风格的健康检查端点。
//
// - /health ：liveness 探针。进程存活即 200（不检查任何依赖）。
// - /ready  ：readiness 探针。检查数据库连通，不通则返回 503。
//
// 两者均为公开端点（无认证中间件），供外部编排系统探测。
// 输出最少信息，避免泄露内部结构。
type HealthHandler struct {
	db        *gorm.DB
	startedAt time.Time
	version   string
}

func NewHealthHandler(db *gorm.DB, version string) *HealthHandler {
	return &HealthHandler{db: db, startedAt: time.Now().UTC(), version: version}
}

// Live 用于 liveness：只要进程能响应就返回 200。
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(stdhttp.StatusOK, gin.H{
		"status":    "live",
		"version":   h.version,
		"uptime":    int(time.Since(h.startedAt).Seconds()),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Ready 用于 readiness：依赖（数据库）不可用时返回 503。
// 新实例启动或数据库短暂失联时，编排系统据此停止转发流量。
func (h *HealthHandler) Ready(c *gin.Context) {
	checks := map[string]string{}
	overallOK := true
	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			checks["database"] = "error: " + err.Error()
			overallOK = false
		} else {
			ctx, cancel := c.Request.Context(), func() {}
			_ = cancel
			if err := sqlDB.PingContext(ctx); err != nil {
				checks["database"] = "ping failed: " + err.Error()
				overallOK = false
			} else {
				checks["database"] = "ok"
			}
		}
	} else {
		checks["database"] = "not configured"
		overallOK = false
	}
	status := stdhttp.StatusOK
	state := "ready"
	if !overallOK {
		status = stdhttp.StatusServiceUnavailable
		state = "not_ready"
	}
	c.JSON(status, gin.H{
		"status":    state,
		"version":   h.version,
		"uptime":    int(time.Since(h.startedAt).Seconds()),
		"checks":    checks,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
