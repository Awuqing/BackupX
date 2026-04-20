package http

import (
	"fmt"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

// ApiKeyHandler 管理 API Key（admin 专属）。
type ApiKeyHandler struct {
	service      *service.ApiKeyService
	auditService *service.AuditService
}

func NewApiKeyHandler(apiKeyService *service.ApiKeyService, auditService *service.AuditService) *ApiKeyHandler {
	return &ApiKeyHandler{service: apiKeyService, auditService: auditService}
}

func (h *ApiKeyHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, items)
}

func (h *ApiKeyHandler) Create(c *gin.Context) {
	var input service.ApiKeyCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("API_KEY_INVALID", "API Key 参数不合法", err))
		return
	}
	creator := ""
	if username, exists := c.Get(contextUsernameKey); exists {
		if v, ok := username.(string); ok {
			creator = v
		}
	}
	result, err := h.service.Create(c.Request.Context(), creator, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "api_key", "create", "api_key", fmt.Sprintf("%d", result.ApiKey.ID), result.ApiKey.Name,
		fmt.Sprintf("创建 API Key: %s (角色: %s)", result.ApiKey.Name, result.ApiKey.Role))
	response.Success(c, result)
}

func (h *ApiKeyHandler) Revoke(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.Revoke(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "api_key", "revoke", "api_key", fmt.Sprintf("%d", id), "",
		fmt.Sprintf("撤销 API Key (ID: %d)", id))
	response.Success(c, gin.H{"revoked": true})
}

func (h *ApiKeyHandler) Toggle(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		Disabled bool `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("API_KEY_INVALID", "参数不合法", err))
		return
	}
	if err := h.service.ToggleDisabled(c.Request.Context(), id, input.Disabled); err != nil {
		response.Error(c, err)
		return
	}
	action := "enable"
	label := "启用"
	if input.Disabled {
		action = "disable"
		label = "停用"
	}
	recordAudit(c, h.auditService, "api_key", action, "api_key", fmt.Sprintf("%d", id), "",
		fmt.Sprintf("%s API Key (ID: %d)", label, id))
	response.Success(c, gin.H{"disabled": input.Disabled})
}
