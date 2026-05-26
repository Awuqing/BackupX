package http

import (
	stdhttp "net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// resolveWebRoot 返回前端静态资源目录。优先使用显式配置的路径，
// 否则按部署惯例依次探测常见位置，返回首个包含 index.html 的目录。
// 返回空字符串表示未找到前端产物，此时后端退化为纯 API 服务。
func resolveWebRoot(configured string) string {
	candidates := []string{
		configured,
		"./web/dist",  // 源码树根目录构建产物（优先于 ./web，避免命中前端源码模板）
		"./web",       // systemd：WorkingDirectory=/opt/backupx → /opt/backupx/web；容器 WORKDIR=/app → /app/web
		"../web/dist", // 从 server/ 目录运行（make dev-server）
		"/opt/backupx/web",
		"/app/web",
	}
	for _, dir := range candidates {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		if hasIndexHTML(dir) {
			if abs, err := filepath.Abs(dir); err == nil {
				return abs
			}
			return dir
		}
	}
	return ""
}

func hasIndexHTML(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil && !info.IsDir()
}

// isReservedBackendPath 判断请求是否命中后端保留前缀（API、探针、安装脚本）。
// 这些路径即使未匹配到具体路由，也应返回结构化 JSON 404，而不是回退到
// 前端 index.html —— 否则反向代理/安装脚本会把 HTML 当成接口响应（参考 issue #46）。
func isReservedBackendPath(p string) bool {
	switch p {
	case "/health", "/ready", "/metrics", "/api", "/install":
		return true
	}
	return strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/install/")
}

// spaFileServer 构造 SPA 静态资源处理器，用作 gin 的 NoRoute 回退：
//   - 后端保留前缀返回 apiNotFound（JSON 404）；
//   - 其余 GET/HEAD 请求若在 webRoot 内命中真实文件则直接返回该文件；
//   - 未命中文件的路径回退到 index.html，交由前端路由处理（history 模式刷新）。
func spaFileServer(webRoot string, apiNotFound gin.HandlerFunc) gin.HandlerFunc {
	indexPath := filepath.Join(webRoot, "index.html")
	return func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		if isReservedBackendPath(reqPath) {
			apiNotFound(c)
			return
		}
		if c.Request.Method != stdhttp.MethodGet && c.Request.Method != stdhttp.MethodHead {
			apiNotFound(c)
			return
		}

		// 防目录穿越：以 webRoot 为根清理路径，确保最终目标仍位于 webRoot 内。
		clean := filepath.Clean("/" + strings.TrimPrefix(reqPath, "/"))
		target := filepath.Join(webRoot, clean)
		if rel, err := filepath.Rel(webRoot, target); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			apiNotFound(c)
			return
		}

		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			c.File(target)
			return
		}
		// 前端 SPA 路由（/dashboard、/tasks 等）回退到 index.html。
		c.File(indexPath)
	}
}
