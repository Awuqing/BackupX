package http

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

// ReplicationHandler 管理备份复制记录列表 + 手动触发。
type ReplicationHandler struct {
	service      *service.ReplicationService
	auditService *service.AuditService
}

func NewReplicationHandler(replicationService *service.ReplicationService, auditService *service.AuditService) *ReplicationHandler {
	return &ReplicationHandler{service: replicationService, auditService: auditService}
}

// TriggerByRecord 手动触发：从备份记录复制到指定目标存储。
// Body: {"destTargetId": 12}
func (h *ReplicationHandler) TriggerByRecord(c *gin.Context) {
	recordID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		DestTargetID uint `json:"destTargetId" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("REPLICATION_INVALID", "复制参数不合法", err))
		return
	}
	triggeredBy := ""
	if subject, exists := c.Get(contextUsernameKey); exists {
		if v, ok := subject.(string); ok {
			triggeredBy = v
		}
	}
	if triggeredBy == "" {
		triggeredBy = "manual"
	}
	result, err := h.service.Start(c.Request.Context(), recordID, input.DestTargetID, triggeredBy)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "replication", "manual_run", "backup_record", fmt.Sprintf("%d", recordID), "",
		fmt.Sprintf("手动触发复制（备份记录 #%d → 存储 #%d, 复制记录 #%d）", recordID, input.DestTargetID, result.ID))
	response.Success(c, result)
}

func (h *ReplicationHandler) List(c *gin.Context) {
	filter, err := buildReplicationFilter(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	items, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, items)
}

func (h *ReplicationHandler) Get(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, item)
}

func buildReplicationFilter(c *gin.Context) (service.ReplicationRecordListInput, error) {
	var filter service.ReplicationRecordListInput
	if v := strings.TrimSpace(c.Query("taskId")); v != "" {
		parsed, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return filter, apperror.BadRequest("REPLICATION_FILTER_INVALID", "taskId 不合法", err)
		}
		id := uint(parsed)
		filter.TaskID = &id
	}
	if v := strings.TrimSpace(c.Query("backupRecordId")); v != "" {
		parsed, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return filter, apperror.BadRequest("REPLICATION_FILTER_INVALID", "backupRecordId 不合法", err)
		}
		id := uint(parsed)
		filter.BackupRecordID = &id
	}
	if v := strings.TrimSpace(c.Query("destTargetId")); v != "" {
		parsed, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return filter, apperror.BadRequest("REPLICATION_FILTER_INVALID", "destTargetId 不合法", err)
		}
		id := uint(parsed)
		filter.DestTargetID = &id
	}
	filter.Status = strings.TrimSpace(c.Query("status"))
	if v := strings.TrimSpace(c.Query("dateFrom")); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filter, apperror.BadRequest("REPLICATION_FILTER_INVALID", "dateFrom 必须为 RFC3339", err)
		}
		filter.DateFrom = &parsed
	}
	if v := strings.TrimSpace(c.Query("dateTo")); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filter, apperror.BadRequest("REPLICATION_FILTER_INVALID", "dateTo 必须为 RFC3339", err)
		}
		filter.DateTo = &parsed
	}
	return filter, nil
}
