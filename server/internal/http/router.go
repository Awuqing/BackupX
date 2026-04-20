package http

import (
	"context"
	"errors"
	stdhttp "net/http"

	"backupx/server/internal/apperror"
	"backupx/server/internal/config"
	"backupx/server/internal/metrics"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RouterDependencies struct {
	// Context 控制 handler 启动的后台协程（如 ipLimiter GC）的生命周期。
	// app 应传入随进程退出可取消的 ctx；若为 nil 则退化为 context.Background()。
	Context                  context.Context
	Config                   config.Config
	Version                  string
	Logger                   *zap.Logger
	AuthService              *service.AuthService
	SystemService            *service.SystemService
	StorageTargetService     *service.StorageTargetService
	BackupTaskService        *service.BackupTaskService
	BackupExecutionService   *service.BackupExecutionService
	BackupRecordService      *service.BackupRecordService
	RestoreService           *service.RestoreService
	VerificationService      *service.VerificationService
	ReplicationService       *service.ReplicationService
	TaskTemplateService      *service.TaskTemplateService
	TaskExportService        *service.TaskExportService
	SearchService            *service.SearchService
	EventBroadcaster         *service.EventBroadcaster
	UserService              *service.UserService
	ApiKeyService            *service.ApiKeyService
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
	// DB 注入给健康检查端点做 liveness/readiness 探测。
	DB *gorm.DB
	// Metrics 注入给 /metrics 端点；为 nil 时端点返回 503。
	Metrics *metrics.Metrics
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
	backupRecordHandler := NewBackupRecordHandler(deps.BackupRecordService, deps.RestoreService, deps.AuditService)
	restoreRecordHandler := NewRestoreRecordHandler(deps.RestoreService, deps.AuditService)
	verificationHandler := NewVerificationHandler(deps.VerificationService, deps.AuditService)
	replicationHandler := NewReplicationHandler(deps.ReplicationService, deps.AuditService)
	taskTemplateHandler := NewTaskTemplateHandler(deps.TaskTemplateService, deps.AuditService)
	userHandler := NewUserHandler(deps.UserService, deps.AuditService)
	apiKeyHandler := NewApiKeyHandler(deps.ApiKeyService, deps.AuditService)
	// apiKeyAuth：给 AuthMiddleware 注入 API Key 验证能力。
	// 为 nil 时中间件仅支持 JWT，不影响向后兼容。
	var apiKeyAuth ApiKeyAuthenticator
	if deps.ApiKeyService != nil {
		apiKeyAuth = deps.ApiKeyService
	}
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
			auth.POST("/logout", AuthMiddleware(deps.JWTManager, apiKeyAuth), authHandler.Logout)
			auth.GET("/profile", AuthMiddleware(deps.JWTManager, apiKeyAuth), authHandler.Profile)
			auth.PUT("/password", AuthMiddleware(deps.JWTManager, apiKeyAuth), authHandler.ChangePassword)
		}

		system := api.Group("/system")
		system.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
		system.GET("/info", systemHandler.Info)
		system.GET("/update-check", systemHandler.CheckUpdate)

		storageTargets := api.Group("/storage-targets")
		storageTargets.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
		// 静态路由必须在参数路由 /:id 之前注册，避免 Gin 路由冲突
		storageTargets.GET("", storageTargetHandler.List)
		storageTargets.POST("", RequireNotViewer(), storageTargetHandler.Create)
		storageTargets.POST("/test", RequireNotViewer(), storageTargetHandler.TestConnection)
		storageTargets.POST("/google-drive/auth-url", RequireNotViewer(), storageTargetHandler.StartGoogleDriveOAuth)
		storageTargets.POST("/google-drive/complete", RequireNotViewer(), storageTargetHandler.CompleteGoogleDriveOAuth)
		storageTargets.GET("/google-drive/callback", storageTargetHandler.HandleGoogleDriveCallback)
		rcloneHandler := NewRcloneHandler()
		storageTargets.GET("/rclone/backends", rcloneHandler.ListBackends)
		// 参数路由
		storageTargets.GET("/:id", storageTargetHandler.Get)
		storageTargets.PUT("/:id", RequireNotViewer(), storageTargetHandler.Update)
		storageTargets.DELETE("/:id", RequireNotViewer(), storageTargetHandler.Delete)
		storageTargets.PUT("/:id/star", RequireNotViewer(), storageTargetHandler.ToggleStar)
		storageTargets.POST("/:id/test", RequireNotViewer(), storageTargetHandler.TestSavedConnection)
		storageTargets.GET("/:id/usage", storageTargetHandler.GetUsage)
		storageTargets.GET("/:id/google-drive/profile", storageTargetHandler.GoogleDriveProfile)

		backupTasks := api.Group("/backup/tasks")
		backupTasks.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
		backupTasks.GET("", backupTaskHandler.List)
		backupTasks.GET("/tags", backupTaskHandler.ListTags)
		backupTasks.GET("/:id", backupTaskHandler.Get)
		backupTasks.POST("", RequireNotViewer(), backupTaskHandler.Create)
		backupTasks.PUT("/:id", RequireNotViewer(), backupTaskHandler.Update)
		backupTasks.DELETE("/:id", RequireNotViewer(), backupTaskHandler.Delete)
		backupTasks.PUT("/:id/toggle", RequireNotViewer(), backupTaskHandler.Toggle)
		backupTasks.POST("/:id/run", RequireNotViewer(), backupRunHandler.Run)
		backupTasks.POST("/batch/toggle", RequireNotViewer(), backupTaskHandler.BatchToggle)
		backupTasks.POST("/batch/delete", RequireNotViewer(), backupTaskHandler.BatchDelete)
		backupTasks.POST("/batch/run", RequireNotViewer(), backupRunHandler.BatchRun)
		// 任务配置导入/导出（集群迁移 & 灾备）
		if deps.TaskExportService != nil {
			taskExportHandler := NewTaskExportHandler(deps.TaskExportService, deps.AuditService)
			backupTasks.GET("/export", taskExportHandler.Export)
			backupTasks.POST("/import", RequireNotViewer(), taskExportHandler.Import)
		}
		if deps.VerificationService != nil {
			backupTasks.POST("/:id/verify", RequireNotViewer(), verificationHandler.TriggerByTask)
		}

		backupRecords := api.Group("/backup/records")
		backupRecords.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
		backupRecords.GET("", backupRecordHandler.List)
		backupRecords.GET("/:id", backupRecordHandler.Get)
		backupRecords.GET("/:id/logs/stream", backupRecordHandler.StreamLogs)
		backupRecords.GET("/:id/download", backupRecordHandler.Download)
		backupRecords.POST("/:id/restore", RequireNotViewer(), backupRecordHandler.Restore)
		backupRecords.POST("/batch-delete", RequireNotViewer(), backupRecordHandler.BatchDelete)
		backupRecords.DELETE("/:id", RequireNotViewer(), backupRecordHandler.Delete)

		// 恢复记录独立命名空间：列表/详情/SSE 日志流。
		// 创建恢复仍然走 POST /backup/records/:id/restore（以源备份记录为触发点）。
		if deps.RestoreService != nil {
			restoreRecords := api.Group("/restore/records")
			restoreRecords.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
			restoreRecords.GET("", restoreRecordHandler.List)
			restoreRecords.GET("/:id", restoreRecordHandler.Get)
			restoreRecords.GET("/:id/logs/stream", restoreRecordHandler.StreamLogs)
		}

		// 备份复制记录（3-2-1 规则）
		if deps.ReplicationService != nil {
			replicationRecords := api.Group("/replication/records")
			replicationRecords.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
			replicationRecords.GET("", replicationHandler.List)
			replicationRecords.GET("/:id", replicationHandler.Get)
			backupRecords.POST("/:id/replicate", RequireNotViewer(), replicationHandler.TriggerByRecord)
		}

		// 任务模板（批量创建）
		if deps.TaskTemplateService != nil {
			templates := api.Group("/task-templates")
			templates.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
			templates.GET("", taskTemplateHandler.List)
			templates.GET("/:id", taskTemplateHandler.Get)
			templates.POST("", RequireNotViewer(), taskTemplateHandler.Create)
			templates.PUT("/:id", RequireNotViewer(), taskTemplateHandler.Update)
			templates.DELETE("/:id", RequireNotViewer(), taskTemplateHandler.Delete)
			templates.POST("/:id/apply", RequireNotViewer(), taskTemplateHandler.Apply)
		}

		// 备份验证/演练记录
		if deps.VerificationService != nil {
			verifyRecords := api.Group("/verify/records")
			verifyRecords.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
			verifyRecords.GET("", verificationHandler.List)
			verifyRecords.GET("/:id", verificationHandler.Get)
			verifyRecords.GET("/:id/logs/stream", verificationHandler.StreamLogs)
			// 基于备份记录的验证入口：与 restore 对称
			backupRecords.POST("/:id/verify", RequireNotViewer(), verificationHandler.TriggerByRecord)
		}
		dashboard := api.Group("/dashboard")
		dashboard.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
		dashboard.GET("/stats", dashboardHandler.Stats)
		dashboard.GET("/timeline", dashboardHandler.Timeline)
		dashboard.GET("/sla", dashboardHandler.SLA)
		dashboard.GET("/cluster", dashboardHandler.Cluster)
		dashboard.GET("/breakdown", dashboardHandler.Breakdown)
		dashboard.GET("/node-performance", dashboardHandler.NodePerformance)

		notifications := api.Group("/notifications")
		notifications.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
		notifications.GET("", notificationHandler.List)
		notifications.GET("/:id", notificationHandler.Get)
		notifications.POST("", RequireNotViewer(), notificationHandler.Create)
		notifications.PUT("/:id", RequireNotViewer(), notificationHandler.Update)
		notifications.DELETE("/:id", RequireNotViewer(), notificationHandler.Delete)
		notifications.POST("/test", RequireNotViewer(), notificationHandler.Test)
		notifications.POST("/:id/test", RequireNotViewer(), notificationHandler.TestSaved)

		settings := api.Group("/settings")
		settings.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
		settings.GET("", settingsHandler.Get)
		settings.PUT("", RequireRole("admin"), settingsHandler.Update)

		// 用户管理（admin 专属）
		if deps.UserService != nil {
			users := api.Group("/users")
			users.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth), RequireRole("admin"))
			users.GET("", userHandler.List)
			users.POST("", userHandler.Create)
			users.PUT("/:id", userHandler.Update)
			users.DELETE("/:id", userHandler.Delete)
		}

		// API Key 管理（admin 专属）
		if deps.ApiKeyService != nil {
			apiKeys := api.Group("/api-keys")
			apiKeys.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth), RequireRole("admin"))
			apiKeys.GET("", apiKeyHandler.List)
			apiKeys.POST("", apiKeyHandler.Create)
			apiKeys.PUT("/:id/toggle", apiKeyHandler.Toggle)
			apiKeys.DELETE("/:id", apiKeyHandler.Revoke)
		}

		auditLogs := api.Group("/audit-logs")
		auditLogs.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
		auditLogs.GET("", auditHandler.List)
		auditLogs.GET("/export", auditHandler.Export)

		// 实时事件 SSE 流（Dashboard 自刷新、桌面告警）
		if deps.EventBroadcaster != nil {
			eventsHandler := NewEventsHandler(deps.EventBroadcaster)
			events := api.Group("/events")
			events.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
			events.GET("/stream", eventsHandler.Stream)
		}

		// 全局搜索
		if deps.SearchService != nil {
			searchHandler := NewSearchHandler(deps.SearchService)
			searchGroup := api.Group("/search")
			searchGroup.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
			searchGroup.GET("", searchHandler.Search)
		}

		if deps.DatabaseDiscoveryService != nil {
			databaseHandler := NewDatabaseHandler(deps.DatabaseDiscoveryService)
			database := api.Group("/database")
			database.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
			database.POST("/discover", databaseHandler.Discover)
		}

		nodeHandler := NewNodeHandler(deps.NodeService, deps.AuditService, deps.InstallTokenService, deps.UserRepository, deps.MasterExternalURL)
		nodes := api.Group("/nodes")
		nodes.Use(AuthMiddleware(deps.JWTManager, apiKeyAuth))
		nodes.GET("", nodeHandler.List)
		nodes.GET("/:id", nodeHandler.Get)
		nodes.POST("", RequireRole("admin"), nodeHandler.Create)
		nodes.PUT("/:id", RequireRole("admin"), nodeHandler.Update)
		nodes.DELETE("/:id", RequireRole("admin"), nodeHandler.Delete)
		nodes.GET("/:id/fs/list", nodeHandler.ListDirectory)
			nodes.POST("/batch", RequireRole("admin"), nodeHandler.BatchCreate)
			nodes.POST("/:id/install-tokens", RequireRole("admin"), nodeHandler.CreateInstallToken)
			nodes.POST("/:id/rotate-token", RequireRole("admin"), nodeHandler.RotateToken)
			nodes.GET("/:id/install-script-preview", RequireRole("admin"), nodeHandler.PreviewScript)

		// Agent API（token 认证，无需 JWT）
		if deps.AgentService != nil {
			agentHandler := NewAgentHandler(deps.AgentService, deps.NodeService, deps.RestoreService)
			agent := api.Group("/agent")
			agent.POST("/heartbeat", agentHandler.Heartbeat)
			agent.POST("/commands/poll", agentHandler.Poll)
			agent.POST("/commands/:id/result", agentHandler.SubmitCommandResult)
			agent.GET("/tasks/:id", agentHandler.GetTaskSpec)
			agent.POST("/records/:id", agentHandler.UpdateRecord)
			agent.GET("/restores/:id/spec", agentHandler.GetRestoreSpec)
			agent.POST("/restores/:id", agentHandler.UpdateRestore)

			// Agent v1（安装脚本探活用），仅 Self 端点
			v1Agent := api.Group("/v1/agent")
			v1Agent.GET("/self", agentHandler.Self)
		} else {
			// 未启用 Agent 服务时，保留原有 heartbeat 端点以兼容
			api.POST("/agent/heartbeat", nodeHandler.Heartbeat)
		}
	}

	// 健康检查端点（公开、无认证、低开销）
	// K8s/Swarm/Nomad 等编排系统使用这些端点做 liveness/readiness 探测。
	healthHandler := NewHealthHandler(deps.DB, deps.Version)
	engine.GET("/health", healthHandler.Live)
	engine.GET("/ready", healthHandler.Ready)
	// 在 /api 下也暴露一份，方便反向代理按 path 前缀统一路由
	engine.GET("/api/health", healthHandler.Live)
	engine.GET("/api/ready", healthHandler.Ready)

	// Prometheus /metrics 端点（公开、无认证；内网/反向代理授权即可）。
	// 业内通行做法：/metrics 通常由 Prometheus pull 抓取，不走 API Key。
	if deps.Metrics != nil {
		engine.GET("/metrics", gin.WrapH(deps.Metrics.Handler()))
	}

	// 公开安装路由（不走 JWT 中间件）。
	// 同时注册到 / 和 /api 前缀下：
	//   - /install/:token        保留历史 URL，兼容旧 nginx 部署
	//   - /api/install/:token    新 URL，自动走反向代理的 /api/ 转发规则
	//
	// Issue #46：用户的 nginx 只转发 /api/，/install/* 被 SPA fallback 到 index.html，
	// 返回 HTML 被 sh 解释成 "Syntax error"。使用 /api/install/ 可避开此问题。
	if deps.InstallTokenService != nil {
		gcCtx := deps.Context
		if gcCtx == nil {
			gcCtx = context.Background()
		}
		installHandler := NewInstallHandler(gcCtx, deps.InstallTokenService, deps.AuditService, deps.MasterExternalURL)
		engine.GET("/install/:token", installHandler.Script)
		engine.GET("/install/:token/compose.yml", installHandler.Compose)
		engine.GET("/api/install/:token", installHandler.Script)
		engine.GET("/api/install/:token/compose.yml", installHandler.Compose)
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
