package http

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/backup"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

// RestoreRecordHandler 提供恢复记录列表/详情/实时日志端点。
// 创建恢复由 BackupRecordHandler.Restore 代理到 RestoreService.Start。
type RestoreRecordHandler struct {
	service      *service.RestoreService
	auditService *service.AuditService
}

func NewRestoreRecordHandler(restoreService *service.RestoreService, auditService *service.AuditService) *RestoreRecordHandler {
	return &RestoreRecordHandler{service: restoreService, auditService: auditService}
}

func (h *RestoreRecordHandler) List(c *gin.Context) {
	filter, err := buildRestoreFilter(c)
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

func (h *RestoreRecordHandler) Get(c *gin.Context) {
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

func (h *RestoreRecordHandler) StreamLogs(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	detail, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	events := detail.LogEvents
	completed := detail.Status != "running"
	channel, cancel, err := h.service.SubscribeLogs(c.Request.Context(), id, 64)
	if err != nil {
		response.Error(c, err)
		return
	}
	defer cancel()
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	flusher, ok := c.Writer.(interface{ Flush() })
	if !ok {
		response.Error(c, apperror.Internal("RESTORE_STREAM_UNSUPPORTED", "当前连接不支持日志流", nil))
		return
	}
	for _, event := range events {
		if err := writeRestoreSSEEvent(c.Writer, event); err != nil {
			return
		}
		flusher.Flush()
	}
	if completed {
		return
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case event, ok := <-channel:
			if !ok {
				return
			}
			if err := writeRestoreSSEEvent(c.Writer, event); err != nil {
				return
			}
			flusher.Flush()
			if event.Completed {
				return
			}
		}
	}
}

func buildRestoreFilter(c *gin.Context) (service.RestoreRecordListInput, error) {
	var filter service.RestoreRecordListInput
	if taskIDValue := strings.TrimSpace(c.Query("taskId")); taskIDValue != "" {
		parsed, err := strconv.ParseUint(taskIDValue, 10, 32)
		if err != nil {
			return filter, apperror.BadRequest("RESTORE_RECORD_FILTER_INVALID", "taskId 不合法", err)
		}
		v := uint(parsed)
		filter.TaskID = &v
	}
	if backupValue := strings.TrimSpace(c.Query("backupRecordId")); backupValue != "" {
		parsed, err := strconv.ParseUint(backupValue, 10, 32)
		if err != nil {
			return filter, apperror.BadRequest("RESTORE_RECORD_FILTER_INVALID", "backupRecordId 不合法", err)
		}
		v := uint(parsed)
		filter.BackupRecordID = &v
	}
	if nodeValue := strings.TrimSpace(c.Query("nodeId")); nodeValue != "" {
		parsed, err := strconv.ParseUint(nodeValue, 10, 32)
		if err != nil {
			return filter, apperror.BadRequest("RESTORE_RECORD_FILTER_INVALID", "nodeId 不合法", err)
		}
		v := uint(parsed)
		filter.NodeID = &v
	}
	filter.Status = strings.TrimSpace(c.Query("status"))
	if dateFrom := strings.TrimSpace(c.Query("dateFrom")); dateFrom != "" {
		parsed, err := time.Parse(time.RFC3339, dateFrom)
		if err != nil {
			return filter, apperror.BadRequest("RESTORE_RECORD_FILTER_INVALID", "dateFrom 必须为 RFC3339 时间格式", err)
		}
		filter.DateFrom = &parsed
	}
	if dateTo := strings.TrimSpace(c.Query("dateTo")); dateTo != "" {
		parsed, err := time.Parse(time.RFC3339, dateTo)
		if err != nil {
			return filter, apperror.BadRequest("RESTORE_RECORD_FILTER_INVALID", "dateTo 必须为 RFC3339 时间格式", err)
		}
		filter.DateTo = &parsed
	}
	return filter, nil
}

func writeRestoreSSEEvent(writer io.Writer, event backup.LogEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "event: log\ndata: %s\n\n", payload)
	return err
}
