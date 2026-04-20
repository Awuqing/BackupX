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

// SLA 返回所有启用任务的 SLA 合规视图。用于 Dashboard 企业合规卡片。
func (h *DashboardHandler) SLA(c *gin.Context) {
	payload, err := h.service.SLACompliance(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

// Cluster 返回集群节点概览（在线/离线/过期 Agent 等），用于 Dashboard 卡片。
func (h *DashboardHandler) Cluster(c *gin.Context) {
	payload, err := h.service.ClusterOverview(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

// NodePerformance 返回各节点近 N 天的执行表现（成功率/字节数/平均耗时）。
func (h *DashboardHandler) NodePerformance(c *gin.Context) {
	days := 30
	if v := strings.TrimSpace(c.Query("days")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			days = parsed
		}
	}
	payload, err := h.service.NodePerformance(c.Request.Context(), days)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

// Breakdown 返回按类型/状态/节点/存储分组的统计。
func (h *DashboardHandler) Breakdown(c *gin.Context) {
	days := 30
	if v := strings.TrimSpace(c.Query("days")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			days = parsed
		}
	}
	payload, err := h.service.Breakdown(c.Request.Context(), days)
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
