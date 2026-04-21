package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/backup"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
)

const backupTaskMaskedValue = "********"

type BackupTaskUpsertInput struct {
	Name             string   `json:"name" binding:"required,min=1,max=100"`
	Type             string   `json:"type" binding:"required,oneof=file mysql sqlite postgresql pgsql saphana"`
	Enabled          bool     `json:"enabled"`
	CronExpr         string   `json:"cronExpr" binding:"max=64"`
	SourcePath       string   `json:"sourcePath" binding:"max=500"`
	SourcePaths      []string `json:"sourcePaths"`
	ExcludePatterns  []string `json:"excludePatterns"`
	DBHost           string   `json:"dbHost" binding:"max=255"`
	DBPort           int      `json:"dbPort"`
	DBUser           string   `json:"dbUser" binding:"max=100"`
	DBPassword       string   `json:"dbPassword" binding:"max=255"`
	DBName           string   `json:"dbName" binding:"max=255"`
	DBPath           string   `json:"dbPath" binding:"max=500"`
	StorageTargetID  uint     `json:"storageTargetId"`                       // deprecated: 向后兼容
	StorageTargetIDs []uint   `json:"storageTargetIds"`                      // 新增：多存储目标
	NodeID           uint     `json:"nodeId"`                                // 执行节点（0 = 本机 Master 或节点池）
	// NodePoolTag 节点池标签。NodeID=0 且本字段非空时，调度器动态从 Labels 命中的在线节点中选负载最低者。
	NodePoolTag      string   `json:"nodePoolTag" binding:"max=64"`
	Tags             string   `json:"tags" binding:"max=500"`                // 逗号分隔标签
	RetentionDays    int      `json:"retentionDays"`
	Compression      string   `json:"compression" binding:"omitempty,oneof=gzip none"`
	Encrypt          bool     `json:"encrypt"`
	MaxBackups       int      `json:"maxBackups"`
	// ExtraConfig 类型特有扩展配置（如 SAP HANA 的 backupLevel/backupChannels）
	ExtraConfig map[string]any `json:"extraConfig"`
	// 验证（恢复演练）配置
	VerifyEnabled  bool   `json:"verifyEnabled"`
	VerifyCronExpr string `json:"verifyCronExpr" binding:"max=64"`
	VerifyMode     string `json:"verifyMode" binding:"omitempty,oneof=quick deep"`
	// SLA 配置
	SLAHoursRPO             int `json:"slaHoursRpo"`
	AlertOnConsecutiveFails int `json:"alertOnConsecutiveFails"`
	// 备份复制目标存储 ID 列表（3-2-1 规则）
	ReplicationTargetIDs []uint `json:"replicationTargetIds"`
	// 维护窗口（CSV，详见 backup/window.go）
	MaintenanceWindows string `json:"maintenanceWindows" binding:"max=500"`
	// 依赖的上游任务 ID（上游成功后自动触发本任务）
	DependsOnTaskIDs []uint `json:"dependsOnTaskIds"`
}

type BackupTaskToggleInput struct {
	Enabled *bool `json:"enabled"`
}

type BackupTaskSummary struct {
	ID                 uint       `json:"id"`
	Name               string     `json:"name"`
	Type               string     `json:"type"`
	Enabled            bool       `json:"enabled"`
	CronExpr           string     `json:"cronExpr"`
	StorageTargetID    uint       `json:"storageTargetId"`              // deprecated: 取第一个
	StorageTargetName  string     `json:"storageTargetName"`            // deprecated: 取第一个
	StorageTargetIDs   []uint     `json:"storageTargetIds"`
	StorageTargetNames []string   `json:"storageTargetNames"`
	NodeID             uint       `json:"nodeId"`
	NodeName           string     `json:"nodeName,omitempty"`
	NodePoolTag        string     `json:"nodePoolTag,omitempty"`
	Tags               string     `json:"tags"`
	RetentionDays      int        `json:"retentionDays"`
	Compression        string     `json:"compression"`
	Encrypt            bool       `json:"encrypt"`
	MaxBackups         int        `json:"maxBackups"`
	LastRunAt          *time.Time `json:"lastRunAt,omitempty"`
	LastStatus         string     `json:"lastStatus"`
	// 验证与 SLA 元信息
	VerifyEnabled           bool   `json:"verifyEnabled"`
	VerifyCronExpr          string `json:"verifyCronExpr"`
	VerifyMode              string `json:"verifyMode"`
	SLAHoursRPO             int    `json:"slaHoursRpo"`
	AlertOnConsecutiveFails int    `json:"alertOnConsecutiveFails"`
	// 备份复制目标（3-2-1）
	ReplicationTargetIDs []uint `json:"replicationTargetIds"`
	MaintenanceWindows   string `json:"maintenanceWindows"`
	DependsOnTaskIDs     []uint `json:"dependsOnTaskIds"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type BackupTaskDetail struct {
	BackupTaskSummary
	SourcePath      string         `json:"sourcePath"`
	SourcePaths     []string       `json:"sourcePaths"`
	ExcludePatterns []string       `json:"excludePatterns"`
	DBHost          string         `json:"dbHost"`
	DBPort          int            `json:"dbPort"`
	DBUser          string         `json:"dbUser"`
	DBName          string         `json:"dbName"`
	DBPath          string         `json:"dbPath"`
	ExtraConfig     map[string]any `json:"extraConfig,omitempty"`
	MaskedFields    []string       `json:"maskedFields,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
}

type BackupTaskScheduler interface {
	SyncTask(ctx context.Context, task *model.BackupTask) error
	RemoveTask(ctx context.Context, taskID uint) error
}

type BackupTaskService struct {
	tasks           repository.BackupTaskRepository
	targets         repository.StorageTargetRepository
	records         repository.BackupRecordRepository
	nodes           repository.NodeRepository
	storageRegistry *storage.Registry
	cipher          *codec.ConfigCipher
	scheduler       BackupTaskScheduler
}

func NewBackupTaskService(
	tasks repository.BackupTaskRepository,
	targets repository.StorageTargetRepository,
	cipher *codec.ConfigCipher,
) *BackupTaskService {
	return &BackupTaskService{tasks: tasks, targets: targets, cipher: cipher}
}

// SetRecordsAndStorage 注入备份记录仓库和存储注册表，用于任务删除时清理远端文件。
func (s *BackupTaskService) SetRecordsAndStorage(records repository.BackupRecordRepository, registry *storage.Registry) {
	s.records = records
	s.storageRegistry = registry
}

// SetNodeRepository 注入节点仓库用于校验任务绑定的 NodeID 合法。
func (s *BackupTaskService) SetNodeRepository(nodes repository.NodeRepository) {
	s.nodes = nodes
}

func (s *BackupTaskService) SetScheduler(scheduler BackupTaskScheduler) {
	s.scheduler = scheduler
}

func (s *BackupTaskService) List(ctx context.Context) ([]BackupTaskSummary, error) {
	items, err := s.tasks.List(ctx, repository.BackupTaskListOptions{})
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_LIST_FAILED", "无法获取备份任务列表", err)
	}
	result := make([]BackupTaskSummary, 0, len(items))
	for _, item := range items {
		result = append(result, toBackupTaskSummary(&item))
	}
	return result, nil
}

// ListTags 返回全系统所有任务使用过的唯一标签。
func (s *BackupTaskService) ListTags(ctx context.Context) ([]string, error) {
	tags, err := s.tasks.DistinctTags(ctx)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_TAG_LIST_FAILED", "无法获取任务标签", err)
	}
	return tags, nil
}

// BatchResult 单条批量操作结果。best-effort：失败不中断其他。
type BatchResult struct {
	ID      uint   `json:"id"`
	Name    string `json:"name,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// BatchToggle 批量启停任务。
func (s *BackupTaskService) BatchToggle(ctx context.Context, ids []uint, enabled bool) []BatchResult {
	results := make([]BatchResult, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		summary, err := s.Toggle(ctx, id, enabled)
		item := BatchResult{ID: id, Success: err == nil}
		if err != nil {
			item.Error = appErrorMessage(err)
		} else if summary != nil {
			item.Name = summary.Name
		}
		results = append(results, item)
	}
	return results
}

// BatchDeleteTasks 批量删除任务。
func (s *BackupTaskService) BatchDeleteTasks(ctx context.Context, ids []uint) []BatchResult {
	results := make([]BatchResult, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		result, err := s.Delete(ctx, id)
		item := BatchResult{ID: id, Success: err == nil}
		if err != nil {
			item.Error = appErrorMessage(err)
		} else if result != nil {
			item.Name = result.TaskName
		}
		results = append(results, item)
	}
	return results
}

// hasCyclicDependency DFS 查找是否存在从 candidate 上游链回到 taskID 的路径。
// 保守实现：遍历 depth 超过 32 视为潜在循环并返回 true。
func (s *BackupTaskService) hasCyclicDependency(ctx context.Context, taskID uint, candidates []uint) bool {
	visited := map[uint]bool{}
	var dfs func(id uint, depth int) bool
	dfs = func(id uint, depth int) bool {
		if depth > 32 {
			return true
		}
		if id == taskID {
			return true
		}
		if visited[id] {
			return false
		}
		visited[id] = true
		upstream, err := s.tasks.FindByID(ctx, id)
		if err != nil || upstream == nil {
			return false
		}
		for _, up := range parseUintCSV(upstream.DependsOnTaskIDs) {
			if dfs(up, depth+1) {
				return true
			}
		}
		return false
	}
	for _, c := range candidates {
		if dfs(c, 0) {
			return true
		}
	}
	return false
}

// TriggerDependents 上游任务成功后找出所有 depends_on 中含有 upstreamID 的下游任务。
// 供 BackupExecutionService 调用，避免后者直接触达 backup_task_repository。
func (s *BackupTaskService) TriggerDependents(ctx context.Context, upstreamID uint) ([]uint, error) {
	items, err := s.tasks.List(ctx, repository.BackupTaskListOptions{})
	if err != nil {
		return nil, err
	}
	var triggers []uint
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		for _, dep := range parseUintCSV(item.DependsOnTaskIDs) {
			if dep == upstreamID {
				triggers = append(triggers, item.ID)
				break
			}
		}
	}
	return triggers, nil
}

// appErrorMessage 提取 apperror 的可读消息，回退到 error.Error()。
func appErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if appErr, ok := err.(*apperror.AppError); ok {
		return appErr.Message
	}
	return err.Error()
}

func (s *BackupTaskService) Get(ctx context.Context, id uint) (*BackupTaskDetail, error) {
	item, err := s.tasks.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_GET_FAILED", "无法获取备份任务详情", err)
	}
	if item == nil {
		return nil, apperror.New(http.StatusNotFound, "BACKUP_TASK_NOT_FOUND", "备份任务不存在", fmt.Errorf("backup task %d not found", id))
	}
	return s.toDetail(item)
}

func (s *BackupTaskService) Create(ctx context.Context, input BackupTaskUpsertInput) (*BackupTaskDetail, error) {
	input.Type = normalizeBackupTaskType(input.Type)
	if err := s.validateInput(ctx, nil, input); err != nil {
		return nil, err
	}
	existing, err := s.tasks.FindByName(ctx, strings.TrimSpace(input.Name))
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_LOOKUP_FAILED", "无法检查备份任务名称", err)
	}
	if existing != nil {
		return nil, apperror.Conflict("BACKUP_TASK_NAME_EXISTS", "备份任务名称已存在", nil)
	}
	item, err := s.buildTask(nil, input)
	if err != nil {
		return nil, err
	}
	if err := s.tasks.Create(ctx, item); err != nil {
		return nil, apperror.Internal("BACKUP_TASK_CREATE_FAILED", "无法创建备份任务", err)
	}
	if s.scheduler != nil {
		if err := s.scheduler.SyncTask(ctx, item); err != nil {
			return nil, apperror.Internal("BACKUP_TASK_SCHEDULE_FAILED", "无法同步备份任务调度", err)
		}
	}
	return s.Get(ctx, item.ID)
}

func (s *BackupTaskService) Update(ctx context.Context, id uint, input BackupTaskUpsertInput) (*BackupTaskDetail, error) {
	input.Type = normalizeBackupTaskType(input.Type)
	existing, err := s.tasks.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_GET_FAILED", "无法获取备份任务详情", err)
	}
	if existing == nil {
		return nil, apperror.New(http.StatusNotFound, "BACKUP_TASK_NOT_FOUND", "备份任务不存在", fmt.Errorf("backup task %d not found", id))
	}
	if err := s.validateInput(ctx, existing, input); err != nil {
		return nil, err
	}
	sameName, err := s.tasks.FindByName(ctx, strings.TrimSpace(input.Name))
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_LOOKUP_FAILED", "无法检查备份任务名称", err)
	}
	if sameName != nil && sameName.ID != existing.ID {
		return nil, apperror.Conflict("BACKUP_TASK_NAME_EXISTS", "备份任务名称已存在", nil)
	}
	item, err := s.buildTask(existing, input)
	if err != nil {
		return nil, err
	}
	item.ID = existing.ID
	item.CreatedAt = existing.CreatedAt
	if err := s.tasks.Update(ctx, item); err != nil {
		return nil, apperror.Internal("BACKUP_TASK_UPDATE_FAILED", "无法更新备份任务", err)
	}
	if s.scheduler != nil {
		if err := s.scheduler.SyncTask(ctx, item); err != nil {
			return nil, apperror.Internal("BACKUP_TASK_SCHEDULE_FAILED", "无法同步备份任务调度", err)
		}
	}
	return s.Get(ctx, item.ID)
}

// DeleteResult 描述任务删除的结果信息，用于审计日志。
type DeleteResult struct {
	TaskName     string
	RecordCount  int
	CleanedFiles int
}

func (s *BackupTaskService) Delete(ctx context.Context, id uint) (*DeleteResult, error) {
	existing, err := s.tasks.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_GET_FAILED", "无法获取备份任务详情", err)
	}
	if existing == nil {
		return nil, apperror.New(http.StatusNotFound, "BACKUP_TASK_NOT_FOUND", "备份任务不存在", fmt.Errorf("backup task %d not found", id))
	}
	if s.scheduler != nil {
		_ = s.scheduler.RemoveTask(ctx, id)
	}

	// 清理远端存储文件（尽力而为，不阻止删除）
	result := &DeleteResult{TaskName: existing.Name}
	result.RecordCount, result.CleanedFiles = s.cleanupRemoteFiles(ctx, id)

	if err := s.tasks.Delete(ctx, id); err != nil {
		return nil, apperror.Internal("BACKUP_TASK_DELETE_FAILED", "无法删除备份任务", err)
	}
	return result, nil
}

// cleanupRemoteFiles 尽力删除任务相关的远端备份文件，返回记录数和成功删除的文件数。
func (s *BackupTaskService) cleanupRemoteFiles(ctx context.Context, taskID uint) (recordCount int, cleanedFiles int) {
	if s.records == nil || s.storageRegistry == nil {
		return 0, 0
	}
	records, err := s.records.ListByTask(ctx, taskID)
	if err != nil {
		return 0, 0
	}
	recordCount = len(records)
	// 缓存 provider 避免同一存储目标重复创建连接
	providerCache := make(map[uint]storage.StorageProvider)
	for _, record := range records {
		if strings.TrimSpace(record.StoragePath) == "" {
			continue
		}
		provider, ok := providerCache[record.StorageTargetID]
		if !ok {
			provider, err = s.resolveStorageProvider(ctx, record.StorageTargetID)
			if err != nil {
				continue
			}
			providerCache[record.StorageTargetID] = provider
		}
		if err := provider.Delete(ctx, record.StoragePath); err == nil {
			cleanedFiles++
		}
	}
	return recordCount, cleanedFiles
}

func (s *BackupTaskService) resolveStorageProvider(ctx context.Context, targetID uint) (storage.StorageProvider, error) {
	target, err := s.targets.FindByID(ctx, targetID)
	if err != nil || target == nil {
		return nil, fmt.Errorf("target %d not found", targetID)
	}
	configMap := map[string]any{}
	if err := s.cipher.DecryptJSON(target.ConfigCiphertext, &configMap); err != nil {
		return nil, err
	}
	provider, err := s.storageRegistry.Create(ctx, target.Type, configMap)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

func (s *BackupTaskService) Toggle(ctx context.Context, id uint, enabled bool) (*BackupTaskSummary, error) {
	item, err := s.tasks.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_GET_FAILED", "无法获取备份任务详情", err)
	}
	if item == nil {
		return nil, apperror.New(http.StatusNotFound, "BACKUP_TASK_NOT_FOUND", "备份任务不存在", fmt.Errorf("backup task %d not found", id))
	}
	item.Enabled = enabled
	if err := s.tasks.Update(ctx, item); err != nil {
		return nil, apperror.Internal("BACKUP_TASK_UPDATE_FAILED", "无法更新备份任务状态", err)
	}
	if s.scheduler != nil {
		if err := s.scheduler.SyncTask(ctx, item); err != nil {
			return nil, apperror.Internal("BACKUP_TASK_SCHEDULE_FAILED", "无法同步备份任务调度", err)
		}
	}
	returnPtr, err := s.tasks.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_GET_FAILED", "无法获取备份任务详情", err)
	}
	returnValue := toBackupTaskSummary(returnPtr)
	return &returnValue, nil
}

// resolveStorageTargetIDs 统一处理新旧字段，返回有效的存储目标 ID 列表
func resolveStorageTargetIDs(input BackupTaskUpsertInput) []uint {
	if len(input.StorageTargetIDs) > 0 {
		return input.StorageTargetIDs
	}
	if input.StorageTargetID > 0 {
		return []uint{input.StorageTargetID}
	}
	return nil
}

func (s *BackupTaskService) validateInput(ctx context.Context, existing *model.BackupTask, input BackupTaskUpsertInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return apperror.BadRequest("BACKUP_TASK_INVALID", "任务名称不能为空", nil)
	}
	targetIDs := resolveStorageTargetIDs(input)
	if len(targetIDs) == 0 {
		return apperror.BadRequest("BACKUP_TASK_INVALID", "请选择至少一个存储目标", nil)
	}
	for _, tid := range targetIDs {
		target, err := s.targets.FindByID(ctx, tid)
		if err != nil {
			return apperror.Internal("BACKUP_TASK_STORAGE_LOOKUP_FAILED", "无法检查存储目标", err)
		}
		if target == nil {
			return apperror.BadRequest("BACKUP_STORAGE_TARGET_INVALID", fmt.Sprintf("关联的存储目标 %d 不存在", tid), nil)
		}
	}
	if input.NodeID > 0 && s.nodes != nil {
		node, err := s.nodes.FindByID(ctx, input.NodeID)
		if err != nil {
			return apperror.Internal("BACKUP_TASK_NODE_LOOKUP_FAILED", "无法校验执行节点", err)
		}
		if node == nil {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "所选执行节点不存在", nil)
		}
	}
	// 节点池与固定节点互斥：固定节点已确定执行位置，不再动态调度
	if input.NodeID > 0 && strings.TrimSpace(input.NodePoolTag) != "" {
		return apperror.BadRequest("BACKUP_TASK_INVALID",
			"固定执行节点与节点池标签只能选其一", nil)
	}
	if input.RetentionDays < 0 {
		return apperror.BadRequest("BACKUP_TASK_INVALID", "保留天数不能小于 0", nil)
	}
	if input.MaxBackups < 0 {
		return apperror.BadRequest("BACKUP_TASK_INVALID", "最大保留份数不能小于 0", nil)
	}
	if input.Compression == "" {
		input.Compression = "gzip"
	}
	if strings.TrimSpace(input.CronExpr) != "" && len(strings.Fields(strings.TrimSpace(input.CronExpr))) < 5 {
		return apperror.BadRequest("BACKUP_TASK_INVALID", "Cron 表达式格式不正确", nil)
	}
	if input.VerifyEnabled {
		if strings.TrimSpace(input.VerifyCronExpr) == "" {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "启用验证演练时必须填写验证 Cron 表达式", nil)
		}
		if len(strings.Fields(strings.TrimSpace(input.VerifyCronExpr))) < 5 {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "验证 Cron 表达式格式不正确", nil)
		}
	}
	if strings.TrimSpace(input.MaintenanceWindows) != "" {
		if err := backup.ValidateMaintenanceWindows(input.MaintenanceWindows); err != nil {
			return apperror.BadRequest("BACKUP_TASK_INVALID", err.Error(), err)
		}
	}
	// 依赖检查：每个上游任务必须存在 + 不能依赖自己 + 无循环
	if len(input.DependsOnTaskIDs) > 0 {
		currentID := uint(0)
		if existing != nil {
			currentID = existing.ID
		}
		for _, dep := range input.DependsOnTaskIDs {
			if dep == 0 {
				continue
			}
			if dep == currentID {
				return apperror.BadRequest("BACKUP_TASK_INVALID", "不能把任务自己设为上游依赖", nil)
			}
			upstream, err := s.tasks.FindByID(ctx, dep)
			if err != nil {
				return apperror.Internal("BACKUP_TASK_DEP_LOOKUP_FAILED", "无法校验上游任务", err)
			}
			if upstream == nil {
				return apperror.BadRequest("BACKUP_TASK_INVALID", fmt.Sprintf("上游任务 %d 不存在", dep), nil)
			}
		}
		if currentID > 0 && s.hasCyclicDependency(ctx, currentID, input.DependsOnTaskIDs) {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "依赖关系会形成循环", nil)
		}
	}
	passwordRequired := existing == nil || existing.DBPasswordCiphertext == ""
	return validateTaskTypeSpecificFields(input, passwordRequired)
}

func validateTaskTypeSpecificFields(input BackupTaskUpsertInput, passwordRequired bool) error {
	switch normalizeBackupTaskType(input.Type) {
	case "file":
		hasSourcePaths := len(resolveSourcePaths(input)) > 0
		if !hasSourcePaths {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "文件备份必须填写源路径", nil)
		}
	case "mysql", "postgresql", "saphana":
		if strings.TrimSpace(input.DBHost) == "" {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "数据库主机不能为空", nil)
		}
		if input.DBPort <= 0 {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "数据库端口必须大于 0", nil)
		}
		if strings.TrimSpace(input.DBUser) == "" {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "数据库用户名不能为空", nil)
		}
		if passwordRequired && strings.TrimSpace(input.DBPassword) == "" {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "数据库密码不能为空", nil)
		}
		if strings.TrimSpace(input.DBName) == "" {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "数据库名称不能为空", nil)
		}
	case "sqlite":
		if strings.TrimSpace(input.DBPath) == "" {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "SQLite 备份必须填写数据库文件路径", nil)
		}
	default:
		return apperror.BadRequest("BACKUP_TASK_INVALID", "不支持的备份任务类型", nil)
	}
	return nil
}

func (s *BackupTaskService) buildTask(existing *model.BackupTask, input BackupTaskUpsertInput) (*model.BackupTask, error) {
	excludePatterns, err := encodeExcludePatterns(input.ExcludePatterns)
	if err != nil {
		return nil, apperror.BadRequest("BACKUP_TASK_INVALID", "排除规则格式不合法", err)
	}
	sourcePathsJSON, err := encodeSourcePaths(resolveSourcePaths(input))
	if err != nil {
		return nil, apperror.BadRequest("BACKUP_TASK_INVALID", "源路径格式不合法", err)
	}
	passwordCiphertext := ""
	if existing != nil {
		passwordCiphertext = existing.DBPasswordCiphertext
	}
	if text := strings.TrimSpace(input.DBPassword); text != "" && text != backupTaskMaskedValue {
		ciphertext, encryptErr := s.cipher.Encrypt([]byte(text))
		if encryptErr != nil {
			return nil, apperror.Internal("BACKUP_TASK_ENCRYPT_FAILED", "无法保存数据库密码", encryptErr)
		}
		passwordCiphertext = ciphertext
	}
	compression := strings.TrimSpace(input.Compression)
	if compression == "" {
		compression = "gzip"
	}
	maxBackups := input.MaxBackups
	if maxBackups == 0 {
		maxBackups = 10
	}
	targetIDs := resolveStorageTargetIDs(input)
	// 保持旧字段兼容：取第一个
	primaryTargetID := uint(0)
	if len(targetIDs) > 0 {
		primaryTargetID = targetIDs[0]
	}
	// 构建多对多关联
	storageTargets := make([]model.StorageTarget, len(targetIDs))
	for i, tid := range targetIDs {
		storageTargets[i] = model.StorageTarget{ID: tid}
	}
	// 向后兼容：SourcePath 取第一个
	resolvedPaths := resolveSourcePaths(input)
	primarySourcePath := strings.TrimSpace(input.SourcePath)
	if len(resolvedPaths) > 0 {
		primarySourcePath = resolvedPaths[0]
	}
	extraConfigJSON, err := encodeExtraConfig(input.ExtraConfig)
	if err != nil {
		return nil, apperror.BadRequest("BACKUP_TASK_INVALID", "扩展配置格式不合法", err)
	}
	item := &model.BackupTask{
		Name:                 strings.TrimSpace(input.Name),
		Type:                 normalizeBackupTaskType(input.Type),
		Enabled:              input.Enabled,
		CronExpr:             strings.TrimSpace(input.CronExpr),
		SourcePath:           primarySourcePath,
		SourcePaths:          sourcePathsJSON,
		ExcludePatterns:      excludePatterns,
		DBHost:               strings.TrimSpace(input.DBHost),
		DBPort:               input.DBPort,
		DBUser:               strings.TrimSpace(input.DBUser),
		DBPasswordCiphertext: passwordCiphertext,
		DBName:               strings.TrimSpace(input.DBName),
		DBPath:               strings.TrimSpace(input.DBPath),
		ExtraConfig:          extraConfigJSON,
		StorageTargetID:      primaryTargetID,
		StorageTargets:       storageTargets,
		NodeID:               input.NodeID,
		NodePoolTag:          strings.TrimSpace(input.NodePoolTag),
		Tags:                 strings.TrimSpace(input.Tags),
		RetentionDays:        input.RetentionDays,
		Compression:          compression,
		Encrypt:              input.Encrypt,
		MaxBackups:           maxBackups,
		LastStatus:           "idle",
		VerifyEnabled:        input.VerifyEnabled,
		VerifyCronExpr:       strings.TrimSpace(input.VerifyCronExpr),
		VerifyMode:           normalizeVerifyMode(input.VerifyMode),
		SLAHoursRPO:          maxInt(0, input.SLAHoursRPO),
		AlertOnConsecutiveFails: alertThreshold(input.AlertOnConsecutiveFails),
		ReplicationTargetIDs: encodeUintCSV(input.ReplicationTargetIDs),
		MaintenanceWindows:   strings.TrimSpace(input.MaintenanceWindows),
		DependsOnTaskIDs:     encodeUintCSV(input.DependsOnTaskIDs),
	}
	if existing != nil {
		item.LastRunAt = existing.LastRunAt
		item.LastStatus = existing.LastStatus
		item.CreatedAt = existing.CreatedAt
	}
	return item, nil
}

func (s *BackupTaskService) toDetail(item *model.BackupTask) (*BackupTaskDetail, error) {
	excludePatterns, err := decodeExcludePatterns(item.ExcludePatterns)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_DECODE_FAILED", "无法解析备份任务配置", err)
	}
	sourcePaths, err := decodeSourcePaths(item.SourcePaths)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_DECODE_FAILED", "无法解析源路径配置", err)
	}
	extraConfig, err := decodeExtraConfig(item.ExtraConfig)
	if err != nil {
		return nil, apperror.Internal("BACKUP_TASK_DECODE_FAILED", "无法解析扩展配置", err)
	}
	detail := &BackupTaskDetail{
		BackupTaskSummary: toBackupTaskSummary(item),
		SourcePath:        item.SourcePath,
		SourcePaths:       sourcePaths,
		ExcludePatterns:   excludePatterns,
		DBHost:            item.DBHost,
		DBPort:            item.DBPort,
		DBUser:            item.DBUser,
		DBName:            item.DBName,
		DBPath:            item.DBPath,
		ExtraConfig:       extraConfig,
		CreatedAt:         item.CreatedAt,
	}
	if item.DBPasswordCiphertext != "" {
		detail.MaskedFields = []string{"dbPassword"}
	}
	return detail, nil
}

func toBackupTaskSummary(item *model.BackupTask) BackupTaskSummary {
	// 从多对多关联提取 IDs 和 Names
	var targetIDs []uint
	var targetNames []string
	if len(item.StorageTargets) > 0 {
		for _, t := range item.StorageTargets {
			targetIDs = append(targetIDs, t.ID)
			targetNames = append(targetNames, t.Name)
		}
	} else if item.StorageTargetID > 0 {
		// 回退到旧字段
		targetIDs = []uint{item.StorageTargetID}
		targetNames = []string{item.StorageTarget.Name}
	}
	// 向后兼容：取第一个
	primaryID := uint(0)
	primaryName := ""
	if len(targetIDs) > 0 {
		primaryID = targetIDs[0]
	}
	if len(targetNames) > 0 {
		primaryName = targetNames[0]
	}
	return BackupTaskSummary{
		ID:                 item.ID,
		Name:               item.Name,
		Type:               normalizeBackupTaskType(item.Type),
		Enabled:            item.Enabled,
		CronExpr:           item.CronExpr,
		StorageTargetID:    primaryID,
		StorageTargetName:  primaryName,
		StorageTargetIDs:   targetIDs,
		StorageTargetNames: targetNames,
		NodeID:             item.NodeID,
		NodeName:           item.Node.Name,
		NodePoolTag:        item.NodePoolTag,
		Tags:               item.Tags,
		RetentionDays:      item.RetentionDays,
		Compression:        item.Compression,
		Encrypt:            item.Encrypt,
		MaxBackups:         item.MaxBackups,
		LastRunAt:          item.LastRunAt,
		LastStatus:         item.LastStatus,
		VerifyEnabled:           item.VerifyEnabled,
		VerifyCronExpr:          item.VerifyCronExpr,
		VerifyMode:              item.VerifyMode,
		SLAHoursRPO:             item.SLAHoursRPO,
		AlertOnConsecutiveFails: item.AlertOnConsecutiveFails,
		ReplicationTargetIDs:    parseUintCSV(item.ReplicationTargetIDs),
		MaintenanceWindows:      item.MaintenanceWindows,
		DependsOnTaskIDs:        parseUintCSV(item.DependsOnTaskIDs),
		UpdatedAt:          item.UpdatedAt,
	}
}

// encodeUintCSV 把 uint 切片编码为 CSV 字符串（去重保序）。
func encodeUintCSV(ids []uint) string {
	if len(ids) == 0 {
		return ""
	}
	seen := map[uint]bool{}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
	}
	return strings.Join(parts, ",")
}

// normalizeVerifyMode 规范化验证模式，未知值默认 quick。
func normalizeVerifyMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "deep":
		return model.VerificationModeDeep
	default:
		return model.VerificationModeQuick
	}
}

// alertThreshold 连续失败告警阈值下限为 1。
func alertThreshold(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func encodeExcludePatterns(value []string) (string, error) {
	if len(value) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodeExcludePatterns(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return []string{}, nil
	}
	var items []string
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return nil, err
	}
	return items, nil
}

// resolveSourcePaths 统一处理 sourcePaths / sourcePath，返回有效路径列表
func resolveSourcePaths(input BackupTaskUpsertInput) []string {
	if len(input.SourcePaths) > 0 {
		var cleaned []string
		for _, p := range input.SourcePaths {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				cleaned = append(cleaned, trimmed)
			}
		}
		if len(cleaned) > 0 {
			return cleaned
		}
	}
	if sp := strings.TrimSpace(input.SourcePath); sp != "" {
		return []string{sp}
	}
	return nil
}

func encodeSourcePaths(paths []string) (string, error) {
	if len(paths) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(paths)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodeSourcePaths(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) == "[]" {
		return []string{}, nil
	}
	var items []string
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func encodeExtraConfig(value map[string]any) (string, error) {
	if len(value) == 0 {
		return "", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodeExtraConfig(value string) (map[string]any, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "{}" {
		return nil, nil
	}
	result := map[string]any{}
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeBackupTaskType(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "pgsql" {
		return "postgresql"
	}
	return normalized
}
