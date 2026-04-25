package http

import (
	"fmt"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

// UserHandler 管理账号（仅 admin 可访问）。
type UserHandler struct {
	service      *service.UserService
	auditService *service.AuditService
}

func NewUserHandler(userService *service.UserService, auditService *service.AuditService) *UserHandler {
	return &UserHandler{service: userService, auditService: auditService}
}

func (h *UserHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, items)
}

func (h *UserHandler) Create(c *gin.Context) {
	var input service.UserUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("USER_INVALID", "用户参数不合法", err))
		return
	}
	item, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "user", "create", "user", fmt.Sprintf("%d", item.ID), item.Username,
		fmt.Sprintf("创建用户 %s (角色: %s)", item.Username, item.Role))
	response.Success(c, item)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input service.UserUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("USER_INVALID", "用户参数不合法", err))
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "user", "update", "user", fmt.Sprintf("%d", id), item.Username,
		fmt.Sprintf("更新用户 %s (角色: %s, 停用: %v)", item.Username, item.Role, item.Disabled))
	response.Success(c, item)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "user", "delete", "user", fmt.Sprintf("%d", id), "",
		fmt.Sprintf("删除用户 (ID: %d)", id))
	response.Success(c, gin.H{"deleted": true})
}

func (h *UserHandler) ResetTwoFactor(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	item, err := h.service.ResetTwoFactor(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "user", "reset_two_factor", "user", fmt.Sprintf("%d", id), item.Username,
		fmt.Sprintf("重置用户 %s 的 MFA", item.Username))
	response.Success(c, item)
}
