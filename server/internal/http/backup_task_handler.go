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
	recordAudit(c, h.auditService, "backup_task", "create", "backup_task", fmt.Sprintf("%d", item.ID), item.Name, fmt.Sprintf("类型: %s", input.Type))
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
	recordAudit(c, h.auditService, "backup_task", "update", "backup_task", fmt.Sprintf("%d", item.ID), item.Name, fmt.Sprintf("类型: %s, Cron: %s", input.Type, input.CronExpr))
	response.Success(c, item)
}

func (h *BackupTaskHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "backup_task", "delete", "backup_task", fmt.Sprintf("%d", id), "", fmt.Sprintf("删除备份任务 #%d", id))
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
	if !enabled {
		action = "disable"
	}
	recordAudit(c, h.auditService, "backup_task", action, "backup_task", fmt.Sprintf("%d", id), item.Name, fmt.Sprintf("%s 备份任务", action))
	response.Success(c, item)
}
