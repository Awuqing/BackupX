package http

import (
	"strconv"
	"strings"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	service *service.DashboardService
}

func NewDashboardHandler(dashboardService *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: dashboardService}
}

func (h *DashboardHandler) Stats(c *gin.Context) {
	payload, err := h.service.Stats(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

func (h *DashboardHandler) Timeline(c *gin.Context) {
	days := 30
	if value := strings.TrimSpace(c.Query("days")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			response.Error(c, apperror.BadRequest("DASHBOARD_TIMELINE_INVALID", "days 必须为整数", err))
			return
		}
		days = parsed
	}
	payload, err := h.service.Timeline(c.Request.Context(), days)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}
