package http

import (
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type SystemHandler struct {
	systemService *service.SystemService
}

func NewSystemHandler(systemService *service.SystemService) *SystemHandler {
	return &SystemHandler{systemService: systemService}
}

func (h *SystemHandler) Info(c *gin.Context) {
	response.Success(c, h.systemService.GetInfo(c.Request.Context()))
}

func (h *SystemHandler) CheckUpdate(c *gin.Context) {
	result, err := h.systemService.CheckUpdate(c.Request.Context())
	if err != nil {
		// 即使检查失败也返回当前版本信息
		response.Success(c, gin.H{
			"currentVersion": result.CurrentVersion,
			"hasUpdate":      false,
			"error":          err.Error(),
		})
		return
	}
	response.Success(c, result)
}
