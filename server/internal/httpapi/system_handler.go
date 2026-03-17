//go:build ignore

package httpapi

import (
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type systemHandler struct {
	service *service.SystemService
}

func newSystemHandler(service *service.SystemService) *systemHandler {
	return &systemHandler{service: service}
}

func (h *systemHandler) registerRoutes(protected gin.IRouter) {
	protected.GET("/system/info", h.info)
}

func (h *systemHandler) info(c *gin.Context) {
	response.Success(c, h.service.GetInfo())
}
