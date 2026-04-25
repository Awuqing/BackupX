package app

import (
	"context"
	"errors"
	"fmt"
	stdhttp "net/http"
	"time"

	"backupx/server/internal/backup"
	backupretention "backupx/server/internal/backup/retention"
	"backupx/server/internal/config"
	"backupx/server/internal/database"
	aphttp "backupx/server/internal/http"
	"backupx/server/internal/logger"
	"backupx/server/internal/metrics"
	"backupx/server/internal/notify"
	"backupx/server/internal/repository"
	"backupx/server/internal/scheduler"
	"backupx/server/internal/security"
	"backupx/server/internal/service"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
	storageRclone "backupx/server/internal/storage/rclone"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Application struct {
	cfg        config.Config
	version    string
	logger     *zap.Logger
	db         *gorm.DB
	httpServer *stdhttp.Server
	scheduler  *scheduler.Service
}

func New(ctx context.Context, cfg config.Config, version string) (*Application, error) {
	appLogger, err := logger.New(cfg.Log)
	if err != nil {
		return nil, fmt.Errorf("init logger: %w", err)
	}

	db, err := database.Open(cfg.Database, appLogger)
	if err != nil {
		return nil, fmt.Errorf("init database: %w", err)
	}

	userRepo := repository.NewUserRepository(db)
	systemConfigRepo := repository.NewSystemConfigRepository(db)
	storageTargetRepo := repository.NewStorageTargetRepository(db)
	backupTaskRepo := repository.NewBackupTaskRepository(db)
	backupRecordRepo := repository.NewBackupRecordRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)
	oauthSessionRepo := repository.NewOAuthSessionRepository(db)
	resolvedSecurity, err := service.ResolveSecurity(ctx, cfg.Security, systemConfigRepo)
	if err != nil {
		return nil, fmt.Errorf("resolve security config: %w", err)
	}

	jwtManager := security.NewJWTManager(resolvedSecurity.JWTSecret, config.MustJWTDuration(cfg.Security))
	rateLimiter := security.NewLoginRateLimiter(5, time.Minute)
	configCipher := codec.NewConfigCipher(resolvedSecurity.EncryptionKey)
	authService := service.NewAuthService(userRepo, systemConfigRepo, jwtManager, rateLimiter, configCipher)
	systemService := service.NewSystemService(cfg, version, time.Now().UTC())
	storageRegistry := storage.NewRegistry(
		storageRclone.NewLocalDiskFactory(),
		storageRclone.NewS3Factory(),
		storageRclone.NewWebDAVFactory(),
		storageRclone.NewGoogleDriveFactory(),
		storageRclone.NewAliyunOSSFactory(),
		storageRclone.NewTencentCOSFactory(),
		storageRclone.NewQiniuKodoFactory(),
		storageRclone.NewFTPFactory(),
		storageRclone.NewRcloneFactory(),
	)
	// 将全部 rclone 后端注册为独立存储类型（sftp、azureblob、dropbox 等与 s3、ftp 完全平级）
	storageRclone.RegisterAllBackends(storageRegistry)
	storageTargetService := service.NewStorageTargetService(storageTargetRepo, oauthSessionRepo, storageRegistry, configCipher)
	storageTargetService.SetBackupTaskRepository(backupTaskRepo)
	storageTargetService.SetBackupRecordRepository(backupRecordRepo)
	backupTaskService := service.NewBackupTaskService(backupTaskRepo, storageTargetRepo, configCipher)
	backupTaskService.SetRecordsAndStorage(backupRecordRepo, storageRegistry)
	// nodeRepo 在下方 Cluster 节点管理区块才实例化，这里延后注入
	backupRunnerRegistry := backup.NewRegistry(backup.NewFileRunner(), backup.NewSQLiteRunner(), backup.NewMySQLRunner(nil), backup.NewPostgreSQLRunner(nil), backup.NewSAPHANARunner(nil))
	logHub := backup.NewLogHub()
	retentionService := backupretention.NewService(backupRecordRepo)
	notifyRegistry := notify.NewRegistry(notify.NewEmailNotifier(), notify.NewWebhookNotifier(), notify.NewTelegramNotifier())
	notificationService := service.NewNotificationService(notificationRepo, notifyRegistry, configCipher)
	authService.SetNotificationService(notificationService)
	// 初始化 rclone 传输配置（重试 + 带宽限制）
	rcloneCtx := storageRclone.ConfiguredContext(ctx, storageRclone.TransferConfig{
		LowLevelRetries: cfg.Backup.Retries,
		BandwidthLimit:  cfg.Backup.BandwidthLimit,
	})
	storageRclone.StartAccounting(rcloneCtx)

	backupExecutionService := service.NewBackupExecutionService(backupTaskRepo, backupRecordRepo, storageTargetRepo, storageRegistry, backupRunnerRegistry, logHub, retentionService, configCipher, notificationService, cfg.Backup.TempDir, cfg.Backup.MaxConcurrent, cfg.Backup.Retries, cfg.Backup.BandwidthLimit)
	schedulerService := scheduler.NewService(backupTaskRepo, backupExecutionService, appLogger)
	backupTaskService.SetScheduler(schedulerService)
	// 审计日志注入延迟到 auditService 创建后（见下方）
	backupRecordService := service.NewBackupRecordService(backupRecordRepo, backupExecutionService, logHub)
	// 恢复服务：使用独立 LogHub 避免恢复记录与备份记录 ID 命名空间冲突
	restoreRecordRepo := repository.NewRestoreRecordRepository(db)
	restoreLogHub := backup.NewLogHub()
	dashboardService := service.NewDashboardService(backupTaskRepo, backupRecordRepo, storageTargetRepo)
	settingsService := service.NewSettingsService(systemConfigRepo)

	// Audit
	auditLogRepo := repository.NewAuditLogRepository(db)
	auditService := service.NewAuditService(auditLogRepo)
	authService.SetAuditService(auditService)
	schedulerService.SetAuditRecorder(auditService)
	// 审计日志外输：启动时用当前 settings 初始化 webhook，后续前端修改立即生效
	settingsService.SetAuditWebhookConfigurer(ctx, auditService)

	// Database discovery（集群依赖在 agentService 创建后注入）
	databaseDiscoveryService := service.NewDatabaseDiscoveryService(backup.NewOSCommandExecutor())

	// Cluster: Node management
	nodeRepo := repository.NewNodeRepository(db)
	backupTaskService.SetNodeRepository(nodeRepo)
	schedulerService.SetNodeRepository(nodeRepo)
	nodeService := service.NewNodeService(nodeRepo, version)
	nodeService.SetTaskRepository(backupTaskRepo)
	if err := nodeService.EnsureLocalNode(ctx); err != nil {
		appLogger.Warn("failed to ensure local node", zap.Error(err))
	}
	// 启动离线检测：每 15s 扫描一次，超过 45s 未心跳的远程节点标记为离线
	nodeService.StartOfflineMonitor(ctx, 15*time.Second)

	// Agent 协议服务：命令队列 + 任务下发 + 记录上报
	agentCmdRepo := repository.NewAgentCommandRepository(db)
	agentService := service.NewAgentService(nodeRepo, backupTaskRepo, backupRecordRepo, storageTargetRepo, agentCmdRepo, configCipher)
	agentService.SetRestoreRepository(restoreRecordRepo)
	agentService.StartCommandTimeoutMonitor(ctx, 30*time.Second, 10*time.Minute)

	// 一键部署：install token service + 后台 GC
	installTokenRepo := repository.NewAgentInstallTokenRepository(db)
	installTokenService := service.NewInstallTokenService(installTokenRepo, nodeRepo)
	installTokenService.StartGC(ctx, time.Hour)

	// 把 Agent 下发能力注入到备份执行服务，实现多节点路由
	backupExecutionService.SetClusterDependencies(nodeRepo, agentService)
	// 启用远程目录浏览：NodeService 通过 AgentService 做同步 RPC
	nodeService.SetAgentRPC(agentService)
	// 启用远程数据库发现：远程节点任务配置时 DatabasePicker 拿到的是节点视角的 DB 列表
	databaseDiscoveryService.SetClusterDependencies(nodeRepo, agentService)

	// 恢复服务：集群感知（本地/远程路由），依赖 agentService 入队
	restoreService := service.NewRestoreService(
		restoreRecordRepo,
		backupRecordRepo,
		backupTaskRepo,
		storageTargetRepo,
		nodeRepo,
		storageRegistry,
		backupRunnerRegistry,
		restoreLogHub,
		configCipher,
		agentService,
		cfg.Backup.TempDir,
		cfg.Backup.MaxConcurrent,
	)

	// 验证服务：定期校验备份可恢复性（企业合规刚需）
	verificationRecordRepo := repository.NewVerificationRecordRepository(db)
	verifyLogHub := backup.NewLogHub()
	verificationService := service.NewVerificationService(
		verificationRecordRepo,
		backupRecordRepo,
		backupTaskRepo,
		storageTargetRepo,
		nodeRepo,
		storageRegistry,
		verifyLogHub,
		configCipher,
		cfg.Backup.TempDir,
		cfg.Backup.MaxConcurrent,
	)
	// 验证失败通知：通过 NotificationService 的事件总线派发 verify_failed
	verificationService.SetNotifier(service.NewVerificationEventNotifier(notificationService))
	// 恢复完成/失败事件派发（restore_success / restore_failed）
	restoreService.SetEventDispatcher(notificationService)
	// 调度器接入验证演练 cron
	schedulerService.SetVerifyRunner(verificationService)

	// 用户管理与 API Key 服务（企业级 RBAC）
	userService := service.NewUserService(userRepo)
	apiKeyRepo := repository.NewApiKeyRepository(db)
	apiKeyService := service.NewApiKeyService(apiKeyRepo)

	// SLA 后台扫描：每 15 分钟扫描违约任务，同任务 6 小时内不重复派发
	dashboardService.StartSLAMonitor(ctx, notificationService, 15*time.Minute, 6*time.Hour)
	// 存储目标健康扫描：每 5 分钟测试启用目标，掉线即告警
	storageTargetService.StartHealthMonitor(ctx, notificationService, 5*time.Minute)

	// 备份复制服务（3-2-1 规则核心）
	replicationRecordRepo := repository.NewReplicationRecordRepository(db)
	replicationService := service.NewReplicationService(
		replicationRecordRepo, backupRecordRepo, storageTargetRepo,
		nodeRepo, storageRegistry, configCipher,
		cfg.Backup.TempDir, cfg.Backup.MaxConcurrent,
	)
	replicationService.SetEventDispatcher(notificationService)
	backupExecutionService.SetReplicationTrigger(replicationService)
	// 备份成功后触发下游依赖任务（任务依赖链工作流）
	backupExecutionService.SetDependentsResolver(backupTaskService)

	// 任务模板（批量创建）
	taskTemplateRepo := repository.NewTaskTemplateRepository(db)
	taskTemplateService := service.NewTaskTemplateService(taskTemplateRepo, backupTaskService)

	// 任务配置导入/导出（JSON，集群迁移 & 灾备）
	taskExportService := service.NewTaskExportService(backupTaskService, backupTaskRepo, storageTargetRepo, nodeRepo)

	// 全局搜索（跨任务/存储/节点/最近记录）
	searchService := service.NewSearchService(backupTaskRepo, backupRecordRepo, storageTargetRepo, nodeRepo)

	// 实时事件广播器（SSE 推送给前端 Dashboard）
	// 注入 notification 后，每次 DispatchEvent 同时 broadcast 到所有 SSE 订阅者
	eventBroadcaster := service.NewEventBroadcaster()
	notificationService.SetBroadcaster(eventBroadcaster)

	// 集群版本监控：每 30 分钟扫描，节点 24 小时内只告警一次
	clusterVersionMonitor := service.NewClusterVersionMonitor(nodeRepo, version)
	clusterVersionMonitor.SetEventDispatcher(notificationService)
	clusterVersionMonitor.Start(ctx, 30*time.Minute, 24*time.Hour)

	// Dashboard 集群概览依赖注入
	dashboardService.SetClusterDependencies(nodeRepo, version)

	// Prometheus 指标采集：Counter/Histogram 由业务服务实时写入；
	// Gauge 类（存储用量、节点在线、SLA 违约）由 Collector 每 30s 异步刷新，
	// 避免 /metrics 请求路径做慢 IO。
	appMetrics := metrics.New(version)
	backupExecutionService.SetMetrics(appMetrics)
	restoreService.SetMetrics(appMetrics)
	verificationService.SetMetrics(appMetrics)
	replicationService.SetMetrics(appMetrics)
	metricsCollector := metrics.NewCollector(
		appMetrics,
		metrics.NewRepoSource(storageTargetRepo, backupRecordRepo, nodeRepo, backupTaskRepo),
		30*time.Second,
	)
	metricsCollector.Start(ctx)

	router := aphttp.NewRouter(aphttp.RouterDependencies{
		Context:                  ctx,
		Config:                   cfg,
		Version:                  version,
		Logger:                   appLogger,
		AuthService:              authService,
		SystemService:            systemService,
		StorageTargetService:     storageTargetService,
		BackupTaskService:        backupTaskService,
		BackupExecutionService:   backupExecutionService,
		BackupRecordService:      backupRecordService,
		RestoreService:           restoreService,
		VerificationService:      verificationService,
		ReplicationService:       replicationService,
		TaskTemplateService:      taskTemplateService,
		TaskExportService:        taskExportService,
		SearchService:            searchService,
		EventBroadcaster:         eventBroadcaster,
		UserService:              userService,
		ApiKeyService:            apiKeyService,
		NotificationService:      notificationService,
		DashboardService:         dashboardService,
		SettingsService:          settingsService,
		NodeService:              nodeService,
		AgentService:             agentService,
		DatabaseDiscoveryService: databaseDiscoveryService,
		AuditService:             auditService,
		JWTManager:               jwtManager,
		UserRepository:           userRepo,
		SystemConfigRepo:         systemConfigRepo,
		InstallTokenService:      installTokenService,
		MasterExternalURL:        "", // 如需覆盖 URL，可扩展 cfg.Server 增字段；目前留空依赖 X-Forwarded-* / Request.Host
		DB:                       db,
		Metrics:                  appMetrics,
	})

	httpServer := &stdhttp.Server{
		Addr:              cfg.Address(),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return &Application{
		cfg:        cfg,
		version:    version,
		logger:     appLogger,
		db:         db,
		httpServer: httpServer,
		scheduler:  schedulerService,
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	if a.scheduler != nil {
		if err := a.scheduler.Start(context.Background()); err != nil {
			return fmt.Errorf("start scheduler: %w", err)
		}
	}
	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("http server listening", zap.String("addr", a.cfg.Address()), zap.String("version", a.version))
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		a.logger.Info("shutdown signal received")
		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		if a.scheduler != nil {
			if err := a.scheduler.Stop(context.Background()); err != nil {
				return fmt.Errorf("stop scheduler: %w", err)
			}
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve http: %w", err)
		}
		return nil
	}
}

func (a *Application) Close() {
	if a.logger != nil {
		_ = a.logger.Sync()
	}
}

func (a *Application) Logger() *zap.Logger {
	return a.logger
}

func ErrorField(err error) zap.Field {
	return zap.Error(err)
}
