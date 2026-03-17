package http

import (
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

func AuthMiddleware(jwtManager *security.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, apperror.Unauthorized("AUTH_REQUIRED", "请先登录", nil))
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := jwtManager.Parse(tokenString)
		if err != nil {
			response.Error(c, apperror.Unauthorized("AUTH_INVALID_TOKEN", "登录状态已失效，请重新登录", err))
			c.Abort()
			return
		}

		c.Set(contextUserSubjectKey, claims.Subject)
		c.Next()
	}
}

func ClientKey(c *gin.Context) string {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		return "unknown"
	}
	return ip
}
