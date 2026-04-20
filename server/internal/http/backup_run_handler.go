package http

import (
	"fmt"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type BackupRunHandler struct {
	service      *service.BackupExecutionService
	auditService *service.AuditService
}

func NewBackupRunHandler(executionService *service.BackupExecutionService, auditService *service.AuditService) *BackupRunHandler {
	return &BackupRunHandler{service: executionService, auditService: auditService}
}

func (h *BackupRunHandler) Run(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	record, err := h.service.RunTaskByID(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "backup_task", "run", "backup_task", fmt.Sprintf("%d", id), "", "手动触发备份")
	response.Success(c, record)
}

// BatchRun 批量触发备份任务。best-effort：单个失败不影响其他。
// Body: {"ids": [1,2,3]}
func (h *BackupRunHandler) BatchRun(c *gin.Context) {
	var input struct {
		IDs []uint `json:"ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("BACKUP_TASK_BATCH_INVALID", "批量执行参数不合法", err))
		return
	}
	results := make([]service.BatchResult, 0, len(input.IDs))
	succ := 0
	for _, id := range input.IDs {
		if id == 0 {
			continue
		}
		_, err := h.service.RunTaskByID(c.Request.Context(), id)
		item := service.BatchResult{ID: id, Success: err == nil}
		if err != nil {
			if appErr, ok := err.(*apperror.AppError); ok {
				item.Error = appErr.Message
			} else {
				item.Error = err.Error()
			}
		} else {
			succ++
		}
		results = append(results, item)
	}
	recordAudit(c, h.auditService, "backup_task", "batch_run", "backup_task", "", "",
		fmt.Sprintf("批量触发备份 %d/%d", succ, len(results)))
	response.Success(c, results)
}
