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
	authService := service.NewAuthService(userRepo, systemConfigRepo, jwtManager, rateLimiter)
	systemService := service.NewSystemService(cfg, version, time.Now().UTC())
	configCipher := codec.NewConfigCipher(resolvedSecurity.EncryptionKey)
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
	backupRunnerRegistry := backup.NewRegistry(backup.NewFileRunner(), backup.NewSQLiteRunner(), backup.NewMySQLRunner(nil), backup.NewPostgreSQLRunner(nil), backup.NewSAPHANARunner(nil))
	logHub := backup.NewLogHub()
	retentionService := backupretention.NewService(backupRecordRepo)
	notifyRegistry := notify.NewRegistry(notify.NewEmailNotifier(), notify.NewWebhookNotifier(), notify.NewTelegramNotifier())
	notificationService := service.NewNotificationService(notificationRepo, notifyRegistry, configCipher)
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
	dashboardService := service.NewDashboardService(backupTaskRepo, backupRecordRepo, storageTargetRepo)
	settingsService := service.NewSettingsService(systemConfigRepo)

	// Audit
	auditLogRepo := repository.NewAuditLogRepository(db)
	auditService := service.NewAuditService(auditLogRepo)
	authService.SetAuditService(auditService)
	schedulerService.SetAuditRecorder(auditService)

	// Database discovery
	databaseDiscoveryService := service.NewDatabaseDiscoveryService(backup.NewOSCommandExecutor())

	// Cluster: Node management
	nodeRepo := repository.NewNodeRepository(db)
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
	agentService.StartCommandTimeoutMonitor(ctx, 30*time.Second, 10*time.Minute)

	// 一键部署：install token service + 后台 GC
	installTokenRepo := repository.NewAgentInstallTokenRepository(db)
	installTokenService := service.NewInstallTokenService(installTokenRepo, nodeRepo)
	installTokenService.StartGC(ctx, time.Hour)

	// 把 Agent 下发能力注入到备份执行服务，实现多节点路由
	backupExecutionService.SetClusterDependencies(nodeRepo, agentService)
	// 启用远程目录浏览：NodeService 通过 AgentService 做同步 RPC
	nodeService.SetAgentRPC(agentService)

	router := aphttp.NewRouter(aphttp.RouterDependencies{
		Config:                 cfg,
		Version:                version,
		Logger:                 appLogger,
		AuthService:            authService,
		SystemService:          systemService,
		StorageTargetService:   storageTargetService,
		BackupTaskService:      backupTaskService,
		BackupExecutionService: backupExecutionService,
		BackupRecordService:    backupRecordService,
		NotificationService:    notificationService,
		DashboardService:       dashboardService,
		SettingsService:        settingsService,
		NodeService:              nodeService,
		AgentService:             agentService,
		DatabaseDiscoveryService: databaseDiscoveryService,
		AuditService:            auditService,
		JWTManager:               jwtManager,
		UserRepository:           userRepo,
		SystemConfigRepo:         systemConfigRepo,
		InstallTokenService:      installTokenService,
		MasterExternalURL:        "", // 如需覆盖 URL，可扩展 cfg.Server 增字段；目前留空依赖 X-Forwarded-* / Request.Host
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
