package http

import (
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type BackupRunHandler struct {
	service *service.BackupExecutionService
}

func NewBackupRunHandler(executionService *service.BackupExecutionService) *BackupRunHandler {
	return &BackupRunHandler{service: executionService}
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
	response.Success(c, record)
}
