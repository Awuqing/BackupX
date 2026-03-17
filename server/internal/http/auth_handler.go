package http

import (
	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) SetupStatus(c *gin.Context) {
	initialized, err := h.authService.SetupStatus(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"initialized": initialized})
}

func (h *AuthHandler) Setup(c *gin.Context) {
	var input service.SetupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_SETUP_INVALID", "初始化参数不合法", err))
		return
	}
	payload, err := h.authService.Setup(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_LOGIN_INVALID", "登录参数不合法", err))
		return
	}
	payload, err := h.authService.Login(c.Request.Context(), input, ClientKey(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

func (h *AuthHandler) Profile(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	user, err := h.authService.GetCurrentUser(c.Request.Context(), subject)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_PASSWORD_INVALID", "参数不合法", err))
		return
	}
	if err := h.authService.ChangePassword(c.Request.Context(), subject, input); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"changed": true})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	response.Success(c, gin.H{"loggedOut": true})
}
