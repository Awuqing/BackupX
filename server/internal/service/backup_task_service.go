package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage/codec"
)

const backupTaskMaskedValue = "********"

type BackupTaskUpsertInput struct {
	Name            string   `json:"name" binding:"required,min=1,max=100"`
	Type            string   `json:"type" binding:"required,oneof=file mysql sqlite postgresql pgsql"`
	Enabled         bool     `json:"enabled"`
	CronExpr        string   `json:"cronExpr" binding:"max=64"`
	SourcePath      string   `json:"sourcePath" binding:"max=500"`
	ExcludePatterns []string `json:"excludePatterns"`
	DBHost          string   `json:"dbHost" binding:"max=255"`
	DBPort          int      `json:"dbPort"`
	DBUser          string   `json:"dbUser" binding:"max=100"`
	DBPassword      string   `json:"dbPassword" binding:"max=255"`
	DBName          string   `json:"dbName" binding:"max=255"`
	DBPath          string   `json:"dbPath" binding:"max=500"`
	StorageTargetID uint     `json:"storageTargetId" binding:"required"`
	RetentionDays   int      `json:"retentionDays"`
	Compression     string   `json:"compression" binding:"omitempty,oneof=gzip none"`
	Encrypt         bool     `json:"encrypt"`
	MaxBackups      int      `json:"maxBackups"`
}

type BackupTaskToggleInput struct {
	Enabled *bool `json:"enabled"`
}

type BackupTaskSummary struct {
	ID                uint       `json:"id"`
	Name              string     `json:"name"`
	Type              string     `json:"type"`
	Enabled           bool       `json:"enabled"`
	CronExpr          string     `json:"cronExpr"`
	StorageTargetID   uint       `json:"storageTargetId"`
	StorageTargetName string     `json:"storageTargetName"`
	RetentionDays     int        `json:"retentionDays"`
	Compression       string     `json:"compression"`
	Encrypt           bool       `json:"encrypt"`
	MaxBackups        int        `json:"maxBackups"`
	LastRunAt         *time.Time `json:"lastRunAt,omitempty"`
	LastStatus        string     `json:"lastStatus"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type BackupTaskDetail struct {
	BackupTaskSummary
	SourcePath      string    `json:"sourcePath"`
	ExcludePatterns []string  `json:"excludePatterns"`
	DBHost          string    `json:"dbHost"`
	DBPort          int       `json:"dbPort"`
	DBUser          string    `json:"dbUser"`
	DBName          string    `json:"dbName"`
	DBPath          string    `json:"dbPath"`
	MaskedFields    []string  `json:"maskedFields,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

type BackupTaskScheduler interface {
	SyncTask(ctx context.Context, task *model.BackupTask) error
	RemoveTask(ctx context.Context, taskID uint) error
}

type BackupTaskService struct {
	tasks     repository.BackupTaskRepository
	targets   repository.StorageTargetRepository
	cipher    *codec.ConfigCipher
	scheduler BackupTaskScheduler
}

func NewBackupTaskService(
	tasks repository.BackupTaskRepository,
	targets repository.StorageTargetRepository,
	cipher *codec.ConfigCipher,
) *BackupTaskService {
	return &BackupTaskService{tasks: tasks, targets: targets, cipher: cipher}
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

func (s *BackupTaskService) Delete(ctx context.Context, id uint) error {
	existing, err := s.tasks.FindByID(ctx, id)
	if err != nil {
		return apperror.Internal("BACKUP_TASK_GET_FAILED", "无法获取备份任务详情", err)
	}
	if existing == nil {
		return apperror.New(http.StatusNotFound, "BACKUP_TASK_NOT_FOUND", "备份任务不存在", fmt.Errorf("backup task %d not found", id))
	}
	if s.scheduler != nil {
		if err := s.scheduler.RemoveTask(ctx, id); err != nil {
			return apperror.Internal("BACKUP_TASK_SCHEDULE_FAILED", "无法移除备份任务调度", err)
		}
	}
	if err := s.tasks.Delete(ctx, id); err != nil {
		return apperror.Internal("BACKUP_TASK_DELETE_FAILED", "无法删除备份任务", err)
	}
	if s.scheduler != nil {
		_ = s.scheduler.RemoveTask(ctx, id)
	}
	return nil
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

func (s *BackupTaskService) validateInput(ctx context.Context, existing *model.BackupTask, input BackupTaskUpsertInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return apperror.BadRequest("BACKUP_TASK_INVALID", "任务名称不能为空", nil)
	}
	if input.StorageTargetID == 0 {
		return apperror.BadRequest("BACKUP_TASK_INVALID", "请选择存储目标", nil)
	}
	target, err := s.targets.FindByID(ctx, input.StorageTargetID)
	if err != nil {
		return apperror.Internal("BACKUP_TASK_STORAGE_LOOKUP_FAILED", "无法检查存储目标", err)
	}
	if target == nil {
		return apperror.BadRequest("BACKUP_STORAGE_TARGET_INVALID", "关联的存储目标不存在", nil)
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
	passwordRequired := existing == nil || existing.DBPasswordCiphertext == ""
	return validateTaskTypeSpecificFields(input, passwordRequired)
}

func validateTaskTypeSpecificFields(input BackupTaskUpsertInput, passwordRequired bool) error {
	switch normalizeBackupTaskType(input.Type) {
	case "file":
		if strings.TrimSpace(input.SourcePath) == "" {
			return apperror.BadRequest("BACKUP_TASK_INVALID", "文件备份必须填写源路径", nil)
		}
	case "mysql", "postgresql":
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
	item := &model.BackupTask{
		Name:                 strings.TrimSpace(input.Name),
		Type:                 normalizeBackupTaskType(input.Type),
		Enabled:              input.Enabled,
		CronExpr:             strings.TrimSpace(input.CronExpr),
		SourcePath:           strings.TrimSpace(input.SourcePath),
		ExcludePatterns:      excludePatterns,
		DBHost:               strings.TrimSpace(input.DBHost),
		DBPort:               input.DBPort,
		DBUser:               strings.TrimSpace(input.DBUser),
		DBPasswordCiphertext: passwordCiphertext,
		DBName:               strings.TrimSpace(input.DBName),
		DBPath:               strings.TrimSpace(input.DBPath),
		StorageTargetID:      input.StorageTargetID,
		RetentionDays:        input.RetentionDays,
		Compression:          compression,
		Encrypt:              input.Encrypt,
		MaxBackups:           maxBackups,
		LastStatus:           "idle",
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
	detail := &BackupTaskDetail{
		BackupTaskSummary: toBackupTaskSummary(item),
		SourcePath:        item.SourcePath,
		ExcludePatterns:   excludePatterns,
		DBHost:            item.DBHost,
		DBPort:            item.DBPort,
		DBUser:            item.DBUser,
		DBName:            item.DBName,
		DBPath:            item.DBPath,
		CreatedAt:         item.CreatedAt,
	}
	if item.DBPasswordCiphertext != "" {
		detail.MaskedFields = []string{"dbPassword"}
	}
	return detail, nil
}

func toBackupTaskSummary(item *model.BackupTask) BackupTaskSummary {
	storageTargetName := ""
	if item != nil {
		storageTargetName = item.StorageTarget.Name
	}
	return BackupTaskSummary{
		ID:                item.ID,
		Name:              item.Name,
		Type:              normalizeBackupTaskType(item.Type),
		Enabled:           item.Enabled,
		CronExpr:          item.CronExpr,
		StorageTargetID:   item.StorageTargetID,
		StorageTargetName: storageTargetName,
		RetentionDays:     item.RetentionDays,
		Compression:       item.Compression,
		Encrypt:           item.Encrypt,
		MaxBackups:        item.MaxBackups,
		LastRunAt:         item.LastRunAt,
		LastStatus:        item.LastStatus,
		UpdatedAt:         item.UpdatedAt,
	}
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

func normalizeBackupTaskType(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "pgsql" {
		return "postgresql"
	}
	return normalized
}
