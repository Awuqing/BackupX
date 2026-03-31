package http

import (
	"fmt"
	"strings"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	settingsService *service.SettingsService
	auditService    *service.AuditService
}

func NewSettingsHandler(settingsService *service.SettingsService, auditService *service.AuditService) *SettingsHandler {
	return &SettingsHandler{settingsService: settingsService, auditService: auditService}
}

func (h *SettingsHandler) Get(c *gin.Context) {
	settings, err := h.settingsService.GetAll(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *SettingsHandler) Update(c *gin.Context) {
	var input map[string]string
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("SETTINGS_INVALID", "设置参数不合法", err))
		return
	}
	settings, err := h.settingsService.Update(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	recordAudit(c, h.auditService, "settings", "update", "settings", "", "", fmt.Sprintf("修改设置: %s", strings.Join(keys, ", ")))
	response.Success(c, settings)
}
