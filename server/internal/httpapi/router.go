//go:build ignore

package httpapi

import (
	"backupx/server/internal/security"
	"backupx/server/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Dependencies struct {
	Logger        *zap.Logger
	AuthService   *service.AuthService
	SystemService *service.SystemService
	JWTManager    *security.JWTManager
	Mode          string
}

func NewRouter(deps Dependencies) *gin.Engine {
	gin.SetMode(deps.Mode)
	router := gin.New()
	router.Use(Recovery(deps.Logger), RequestLogger(deps.Logger))

	api := router.Group("/api")
	authHandler := newAuthHandler(deps.AuthService, deps.Logger)
	systemHandler := newSystemHandler(deps.SystemService)
	protected := api.Group("")
	protected.Use(AuthMiddleware(deps.JWTManager))

	authHandler.registerRoutes(api, protected)
	systemHandler.registerRoutes(protected)
	api.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return router
}
