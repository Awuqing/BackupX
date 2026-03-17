//go:build ignore

package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"backupx/server/internal/apperror"
	"backupx/server/internal/security"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AuthClaims struct {
	UserID   uint
	Username string
	Role     string
}

func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", zap.Any("panic", recovered), zap.String("path", c.Request.URL.Path))
				response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
				c.Abort()
			}
		}()
		c.Next()
	}
}

func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		logger.Info("http request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

func AuthMiddleware(jwtManager *security.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		if authorization == "" || !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
			response.Error(c, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "缺少有效的认证令牌")
			c.Abort()
			return
		}
		tokenValue := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer"))
		if tokenValue == authorization {
			tokenValue = strings.TrimSpace(strings.TrimPrefix(authorization, "bearer"))
		}
		claims, err := jwtManager.Parse(tokenValue)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "认证令牌无效或已过期")
			c.Abort()
			return
		}
		c.Set(claimsContextKey, AuthClaims{UserID: claims.UserID, Username: claims.Username, Role: claims.Role})
		c.Next()
	}
}

func writeError(c *gin.Context, logger *zap.Logger, err error) {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		if appErr.Err != nil {
			logger.Warn("request failed", zap.String("code", appErr.Code), zap.Error(appErr.Err))
		}
		response.Error(c, appErr.Status, appErr.Code, appErr.Message)
		return
	}
	logger.Error("unexpected error", zap.Error(err))
	response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "服务器内部错误")
}

func bindJSON[T any](c *gin.Context, logger *zap.Logger) (*T, error) {
	var payload T
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Warn("bind json failed", zap.Error(err))
		return nil, apperror.Wrap(http.StatusBadRequest, "INVALID_REQUEST", fmt.Sprintf("请求参数错误: %v", err), err)
	}
	return &payload, nil
}
