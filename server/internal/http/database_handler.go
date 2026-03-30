package http

import (
	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type DatabaseHandler struct {
	service *service.DatabaseDiscoveryService
}

func NewDatabaseHandler(service *service.DatabaseDiscoveryService) *DatabaseHandler {
	return &DatabaseHandler{service: service}
}

func (h *DatabaseHandler) Discover(c *gin.Context) {
	var input service.DatabaseDiscoverInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("DATABASE_DISCOVER_INVALID", "数据库发现参数不合法", err))
		return
	}
	result, err := h.service.Discover(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, result)
}
