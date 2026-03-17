//go:build ignore

package httpapi

import (
	"net/http"

	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type authHandler struct {
	service *service.AuthService
	logger  *zap.Logger
}

type setupRequest struct {
	Username    string `json:"username" binding:"required,min=3,max=64"`
	Password    string `json:"password" binding:"required,min=8,max=128"`
	DisplayName string `json:"displayName" binding:"required,min=1,max=128"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

func newAuthHandler(service *service.AuthService, logger *zap.Logger) *authHandler {
	return &authHandler{service: service, logger: logger}
}

func (h *authHandler) registerRoutes(router gin.IRouter, protected gin.IRouter) {
	router.GET("/auth/setup/status", h.getSetupStatus)
	router.POST("/auth/setup", h.setup)
	router.POST("/auth/login", h.login)
	protected.GET("/auth/profile", h.profile)
}

func (h *authHandler) getSetupStatus(c *gin.Context) {
	initialized, err := h.service.GetSetupStatus(c.Request.Context())
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	response.Success(c, gin.H{"initialized": initialized})
}

func (h *authHandler) setup(c *gin.Context) {
	payload, err := bindJSON[setupRequest](c, h.logger)
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	result, err := h.service.Setup(c.Request.Context(), service.SetupInput{
		Username:    payload.Username,
		Password:    payload.Password,
		DisplayName: payload.DisplayName,
	})
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusCreated, response.Envelope{Code: "OK", Message: "success", Data: result})
}

func (h *authHandler) login(c *gin.Context) {
	payload, err := bindJSON[loginRequest](c, h.logger)
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	result, err := h.service.Login(c.Request.Context(), service.LoginInput{
		Username:   payload.Username,
		Password:   payload.Password,
		RemoteAddr: c.ClientIP(),
	})
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	response.Success(c, result)
}

func (h *authHandler) profile(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", "认证信息无效")
		return
	}
	result, err := h.service.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		writeError(c, h.logger, err)
		return
	}
	response.Success(c, result)
}
