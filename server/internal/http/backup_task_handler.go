package http

import (
	"fmt"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type BackupTaskHandler struct {
	service      *service.BackupTaskService
	auditService *service.AuditService
}

// describeTaskInput 提取审计日志中通用的调度和存储目标描述。
func describeTaskInput(input service.BackupTaskUpsertInput) (cronDesc string, storageCount int) {
	cronDesc = "仅手动执行"
	if input.CronExpr != "" {
		cronDesc = input.CronExpr
	}
	storageCount = len(input.StorageTargetIDs)
	if storageCount == 0 && input.StorageTargetID > 0 {
		storageCount = 1
	}
	return
}

func NewBackupTaskHandler(taskService *service.BackupTaskService, auditService *service.AuditService) *BackupTaskHandler {
	return &BackupTaskHandler{service: taskService, auditService: auditService}
}

func (h *BackupTaskHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, items)
}

func (h *BackupTaskHandler) Get(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, item)
}

func (h *BackupTaskHandler) Create(c *gin.Context) {
	var input service.BackupTaskUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("BACKUP_TASK_INVALID", "备份任务参数不合法", err))
		return
	}
	item, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	cronDesc, storageCount := describeTaskInput(input)
	recordAudit(c, h.auditService, "backup_task", "create", "backup_task", fmt.Sprintf("%d", item.ID), item.Name,
		fmt.Sprintf("创建备份任务「%s」，类型: %s, 调度: %s, 存储: %d 个目标", item.Name, input.Type, cronDesc, storageCount))
	response.Success(c, item)
}

func (h *BackupTaskHandler) Update(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input service.BackupTaskUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("BACKUP_TASK_INVALID", "备份任务参数不合法", err))
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	updCronDesc, updStorageCount := describeTaskInput(input)
	recordAudit(c, h.auditService, "backup_task", "update", "backup_task", fmt.Sprintf("%d", item.ID), item.Name,
		fmt.Sprintf("更新备份任务「%s」，类型: %s, 调度: %s, 存储: %d 个目标", item.Name, input.Type, updCronDesc, updStorageCount))
	response.Success(c, item)
}

func (h *BackupTaskHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	result, err := h.service.Delete(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "backup_task", "delete", "backup_task", fmt.Sprintf("%d", id), result.TaskName,
		fmt.Sprintf("删除备份任务「%s」(ID: %d)，关联记录 %d 条，已清理远端文件 %d 个", result.TaskName, id, result.RecordCount, result.CleanedFiles))
	response.Success(c, gin.H{"deleted": true})
}

func (h *BackupTaskHandler) Toggle(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input service.BackupTaskToggleInput
	if err := c.ShouldBindJSON(&input); err != nil && err.Error() != "EOF" {
		response.Error(c, apperror.BadRequest("BACKUP_TASK_TOGGLE_INVALID", "备份任务启停参数不合法", err))
		return
	}
	current, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	enabled := !current.Enabled
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	item, err := h.service.Toggle(c.Request.Context(), id, enabled)
	if err != nil {
		response.Error(c, err)
		return
	}
	action := "enable"
	actionLabel := "启用"
	if !enabled {
		action = "disable"
		actionLabel = "停用"
	}
	recordAudit(c, h.auditService, "backup_task", action, "backup_task", fmt.Sprintf("%d", id), item.Name,
		fmt.Sprintf("%s备份任务「%s」", actionLabel, item.Name))
	response.Success(c, item)
}
