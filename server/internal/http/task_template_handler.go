package http

import (
	"fmt"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

type TaskTemplateHandler struct {
	service      *service.TaskTemplateService
	auditService *service.AuditService
}

func NewTaskTemplateHandler(templateService *service.TaskTemplateService, auditService *service.AuditService) *TaskTemplateHandler {
	return &TaskTemplateHandler{service: templateService, auditService: auditService}
}

func (h *TaskTemplateHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, items)
}

func (h *TaskTemplateHandler) Get(c *gin.Context) {
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

func (h *TaskTemplateHandler) Create(c *gin.Context) {
	var input service.TaskTemplateUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("TASK_TEMPLATE_INVALID", "模板参数不合法", err))
		return
	}
	creator := ""
	if v, ok := c.Get(contextUsernameKey); ok {
		if s, ok := v.(string); ok {
			creator = s
		}
	}
	item, err := h.service.Create(c.Request.Context(), creator, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "task_template", "create", "task_template", fmt.Sprintf("%d", item.ID), item.Name,
		fmt.Sprintf("创建任务模板: %s (类型: %s)", item.Name, item.TaskType))
	response.Success(c, item)
}

func (h *TaskTemplateHandler) Update(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input service.TaskTemplateUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("TASK_TEMPLATE_INVALID", "模板参数不合法", err))
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "task_template", "update", "task_template", fmt.Sprintf("%d", item.ID), item.Name,
		fmt.Sprintf("更新任务模板: %s", item.Name))
	response.Success(c, item)
}

func (h *TaskTemplateHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "task_template", "delete", "task_template", fmt.Sprintf("%d", id), "",
		fmt.Sprintf("删除任务模板 (ID: %d)", id))
	response.Success(c, gin.H{"deleted": true})
}

// Apply 一键批量创建任务。Body: {variables: [{name, sourcePath, ...}, ...]}
func (h *TaskTemplateHandler) Apply(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input service.TaskTemplateApplyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("TASK_TEMPLATE_INVALID", "应用参数不合法", err))
		return
	}
	results, err := h.service.Apply(c.Request.Context(), id, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}
	recordAudit(c, h.auditService, "task_template", "apply", "task_template", fmt.Sprintf("%d", id), "",
		fmt.Sprintf("应用模板批量创建任务（成功 %d/%d）", successCount, len(results)))
	response.Success(c, results)
}
