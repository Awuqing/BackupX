package http

import (
	"context"
	stdhttp "net/http"
	"strings"

	"backupx/server/internal/apperror"
	"backupx/server/internal/security"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles Cross-Origin Resource Sharing for the API.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == stdhttp.MethodOptions {
			c.AbortWithStatus(stdhttp.StatusNoContent)
			return
		}
		c.Next()
	}
}

// ApiKeyAuthenticator 抽象 API Key 验证能力，避免 middleware 直接依赖 service 包。
// 实现方：service.ApiKeyService。未注入时 AuthMiddleware 仍然支持 JWT。
type ApiKeyAuthenticator interface {
	Authenticate(ctx context.Context, rawKey string) (subject string, role string, err error)
}

// AuthMiddleware 支持两种认证方式：
//   - JWT (Authorization: Bearer <jwt>)：交互式用户
//   - API Key (Authorization: Bearer bax_xxx 或 X-Api-Key: bax_xxx)：第三方脚本
//
// JWT 会在 context 中写入 userSubject / userRole / username；
// API Key 会写入 authSubject=api_key:<id> / userRole=<key role>。
func AuthMiddleware(jwtManager *security.JWTManager, apiKeyAuth ApiKeyAuthenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawToken := extractAuthToken(c)
		if rawToken == "" {
			response.Error(c, apperror.Unauthorized("AUTH_REQUIRED", "请先登录", nil))
			c.Abort()
			return
		}
		if apiKeyAuth != nil && strings.HasPrefix(rawToken, "bax_") {
			subject, role, err := apiKeyAuth.Authenticate(c.Request.Context(), rawToken)
			if err != nil {
				response.Error(c, err)
				c.Abort()
				return
			}
			c.Set(contextAuthSubjectKey, subject)
			c.Set(contextUserRoleKey, role)
			c.Set(contextUserSubjectKey, subject)
			c.Set(contextUsernameKey, subject)
			c.Next()
			return
		}
		claims, err := jwtManager.Parse(rawToken)
		if err != nil {
			response.Error(c, apperror.Unauthorized("AUTH_INVALID_TOKEN", "登录状态已失效，请重新登录", err))
			c.Abort()
			return
		}
		c.Set(contextUserSubjectKey, claims.Subject)
		c.Set(contextUserRoleKey, claims.Role)
		c.Set(contextUsernameKey, claims.Username)
		c.Set(contextAuthSubjectKey, "user:"+claims.Subject)
		c.Next()
	}
}

// extractAuthToken 从 Authorization: Bearer 或 X-Api-Key 中提取原始 token。
func extractAuthToken(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	if key := strings.TrimSpace(c.GetHeader("X-Api-Key")); key != "" {
		return key
	}
	return ""
}

// RequireRole 仅放行指定角色，否则返回 403。
// 必须用在 AuthMiddleware 之后。viewer 只读保护、admin 管理端都靠它。
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[strings.ToLower(r)] = true
	}
	return func(c *gin.Context) {
		role, _ := c.Get(contextUserRoleKey)
		roleStr := ""
		if v, ok := role.(string); ok {
			roleStr = strings.ToLower(v)
		}
		if !allowed[roleStr] {
			response.Error(c, apperror.New(403, "AUTH_FORBIDDEN", "当前角色无权执行此操作", nil))
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireNotViewer 是 RequireRole(admin, operator) 的快捷方式，
// 用于任何"写入/变更"类端点，禁止 viewer 触发。
func RequireNotViewer() gin.HandlerFunc {
	return RequireRole("admin", "operator")
}

func ClientKey(c *gin.Context) string {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		return "unknown"
	}
	return ip
}
