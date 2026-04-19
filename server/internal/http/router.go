package http

import (
	"errors"
	stdhttp "net/http"

	"backupx/server/internal/apperror"
	"backupx/server/internal/config"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type RouterDependencies struct {
	Config                   config.Config
	Version                  string
	Logger                   *zap.Logger
	AuthService              *service.AuthService
	SystemService            *service.SystemService
	StorageTargetService     *service.StorageTargetService
	BackupTaskService        *service.BackupTaskService
	BackupExecutionService   *service.BackupExecutionService
	BackupRecordService      *service.BackupRecordService
	NotificationService      *service.NotificationService
	DashboardService         *service.DashboardService
	SettingsService          *service.SettingsService
	NodeService              *service.NodeService
	AgentService             *service.AgentService
	DatabaseDiscoveryService *service.DatabaseDiscoveryService
	AuditService             *service.AuditService
	JWTManager               *security.JWTManager
	UserRepository           repository.UserRepository
	SystemConfigRepo         repository.SystemConfigRepository
	InstallTokenService      *service.InstallTokenService
	MasterExternalURL        string
}

func NewRouter(deps RouterDependencies) *gin.Engine {
	gin.SetMode(deps.Config.Server.Mode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(CORSMiddleware())
	engine.Use(requestLogger(deps.Logger))

	authHandler := NewAuthHandler(deps.AuthService)
	systemHandler := NewSystemHandler(deps.SystemService)
	storageTargetHandler := NewStorageTargetHandler(deps.StorageTargetService, deps.AuditService)
	backupTaskHandler := NewBackupTaskHandler(deps.BackupTaskService, deps.AuditService)
	backupRunHandler := NewBackupRunHandler(deps.BackupExecutionService, deps.AuditService)
	backupRecordHandler := NewBackupRecordHandler(deps.BackupRecordService, deps.AuditService)
	notificationHandler := NewNotificationHandler(deps.NotificationService)
	dashboardHandler := NewDashboardHandler(deps.DashboardService)
	settingsHandler := NewSettingsHandler(deps.SettingsService, deps.AuditService)
	auditHandler := NewAuditHandler(deps.AuditService)

	api := engine.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.GET("/setup/status", authHandler.SetupStatus)
			auth.POST("/setup", authHandler.Setup)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", AuthMiddleware(deps.JWTManager), authHandler.Logout)
			auth.GET("/profile", AuthMiddleware(deps.JWTManager), authHandler.Profile)
			auth.PUT("/password", AuthMiddleware(deps.JWTManager), authHandler.ChangePassword)
		}

		system := api.Group("/system")
		system.Use(AuthMiddleware(deps.JWTManager))
		system.GET("/info", systemHandler.Info)
		system.GET("/update-check", systemHandler.CheckUpdate)

		storageTargets := api.Group("/storage-targets")
		storageTargets.Use(AuthMiddleware(deps.JWTManager))
		// 静态路由必须在参数路由 /:id 之前注册，避免 Gin 路由冲突
		storageTargets.GET("", storageTargetHandler.List)
		storageTargets.POST("", storageTargetHandler.Create)
		storageTargets.POST("/test", storageTargetHandler.TestConnection)
		storageTargets.POST("/google-drive/auth-url", storageTargetHandler.StartGoogleDriveOAuth)
		storageTargets.POST("/google-drive/complete", storageTargetHandler.CompleteGoogleDriveOAuth)
		storageTargets.GET("/google-drive/callback", storageTargetHandler.HandleGoogleDriveCallback)
		rcloneHandler := NewRcloneHandler()
		storageTargets.GET("/rclone/backends", rcloneHandler.ListBackends)
		// 参数路由
		storageTargets.GET("/:id", storageTargetHandler.Get)
		storageTargets.PUT("/:id", storageTargetHandler.Update)
		storageTargets.DELETE("/:id", storageTargetHandler.Delete)
		storageTargets.PUT("/:id/star", storageTargetHandler.ToggleStar)
		storageTargets.POST("/:id/test", storageTargetHandler.TestSavedConnection)
		storageTargets.GET("/:id/usage", storageTargetHandler.GetUsage)
		storageTargets.GET("/:id/google-drive/profile", storageTargetHandler.GoogleDriveProfile)

		backupTasks := api.Group("/backup/tasks")
		backupTasks.Use(AuthMiddleware(deps.JWTManager))
		backupTasks.GET("", backupTaskHandler.List)
		backupTasks.GET("/:id", backupTaskHandler.Get)
		backupTasks.POST("", backupTaskHandler.Create)
		backupTasks.PUT("/:id", backupTaskHandler.Update)
		backupTasks.DELETE("/:id", backupTaskHandler.Delete)
		backupTasks.PUT("/:id/toggle", backupTaskHandler.Toggle)
		backupTasks.POST("/:id/run", backupRunHandler.Run)

		backupRecords := api.Group("/backup/records")
		backupRecords.Use(AuthMiddleware(deps.JWTManager))
		backupRecords.GET("", backupRecordHandler.List)
		backupRecords.GET("/:id", backupRecordHandler.Get)
		backupRecords.GET("/:id/logs/stream", backupRecordHandler.StreamLogs)
		backupRecords.GET("/:id/download", backupRecordHandler.Download)
		backupRecords.POST("/:id/restore", backupRecordHandler.Restore)
		backupRecords.POST("/batch-delete", backupRecordHandler.BatchDelete)
		backupRecords.DELETE("/:id", backupRecordHandler.Delete)
		dashboard := api.Group("/dashboard")
		dashboard.Use(AuthMiddleware(deps.JWTManager))
		dashboard.GET("/stats", dashboardHandler.Stats)
		dashboard.GET("/timeline", dashboardHandler.Timeline)

		notifications := api.Group("/notifications")
		notifications.Use(AuthMiddleware(deps.JWTManager))
		notifications.GET("", notificationHandler.List)
		notifications.GET("/:id", notificationHandler.Get)
		notifications.POST("", notificationHandler.Create)
		notifications.PUT("/:id", notificationHandler.Update)
		notifications.DELETE("/:id", notificationHandler.Delete)
		notifications.POST("/test", notificationHandler.Test)
		notifications.POST("/:id/test", notificationHandler.TestSaved)

		settings := api.Group("/settings")
		settings.Use(AuthMiddleware(deps.JWTManager))
		settings.GET("", settingsHandler.Get)
		settings.PUT("", settingsHandler.Update)

		auditLogs := api.Group("/audit-logs")
		auditLogs.Use(AuthMiddleware(deps.JWTManager))
		auditLogs.GET("", auditHandler.List)

		if deps.DatabaseDiscoveryService != nil {
			databaseHandler := NewDatabaseHandler(deps.DatabaseDiscoveryService)
			database := api.Group("/database")
			database.Use(AuthMiddleware(deps.JWTManager))
			database.POST("/discover", databaseHandler.Discover)
		}

		nodeHandler := NewNodeHandler(deps.NodeService, deps.AuditService, deps.InstallTokenService, deps.MasterExternalURL)
		nodes := api.Group("/nodes")
		nodes.Use(AuthMiddleware(deps.JWTManager))
		nodes.GET("", nodeHandler.List)
		nodes.GET("/:id", nodeHandler.Get)
		nodes.POST("", nodeHandler.Create)
		nodes.PUT("/:id", nodeHandler.Update)
		nodes.DELETE("/:id", nodeHandler.Delete)
		nodes.GET("/:id/fs/list", nodeHandler.ListDirectory)
			nodes.POST("/batch", nodeHandler.BatchCreate)
			nodes.POST("/:id/install-tokens", nodeHandler.CreateInstallToken)
			nodes.POST("/:id/rotate-token", nodeHandler.RotateToken)
			nodes.GET("/:id/install-script-preview", nodeHandler.PreviewScript)

		// Agent API（token 认证，无需 JWT）
		if deps.AgentService != nil {
			agentHandler := NewAgentHandler(deps.AgentService, deps.NodeService)
			agent := api.Group("/agent")
			agent.POST("/heartbeat", agentHandler.Heartbeat)
			agent.POST("/commands/poll", agentHandler.Poll)
			agent.POST("/commands/:id/result", agentHandler.SubmitCommandResult)
			agent.GET("/tasks/:id", agentHandler.GetTaskSpec)
			agent.POST("/records/:id", agentHandler.UpdateRecord)

			// Agent v1（安装脚本探活用），仅 Self 端点
			v1Agent := api.Group("/v1/agent")
			v1Agent.GET("/self", agentHandler.Self)
		} else {
			// 未启用 Agent 服务时，保留原有 heartbeat 端点以兼容
			api.POST("/agent/heartbeat", nodeHandler.Heartbeat)
		}
	}

	// 公开安装路由（不走 JWT 中间件）
	if deps.InstallTokenService != nil {
		installHandler := NewInstallHandler(deps.InstallTokenService, deps.AuditService, deps.MasterExternalURL)
		engine.GET("/install/:token", installHandler.Script)
		engine.GET("/install/:token/compose.yml", installHandler.Compose)
	}

	engine.NoRoute(func(c *gin.Context) {
		response.Error(c, apperror.New(stdhttp.StatusNotFound, "NOT_FOUND", "接口不存在", errors.New("route not found")))
	})

	return engine
}

func requestLogger(logger *zap.Logger) gin.HandlerFunc {
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
