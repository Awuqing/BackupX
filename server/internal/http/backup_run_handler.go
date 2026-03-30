package http

import (
	"fmt"

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
