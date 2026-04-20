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

// VerificationHandler 提供验证记录列表/详情/SSE，以及手动触发入口。
type VerificationHandler struct {
	service      *service.VerificationService
	auditService *service.AuditService
}

func NewVerificationHandler(verifyService *service.VerificationService, auditService *service.AuditService) *VerificationHandler {
	return &VerificationHandler{service: verifyService, auditService: auditService}
}

// TriggerByTask 接收任务级手动触发。使用最新成功备份为源。
func (h *VerificationHandler) TriggerByTask(c *gin.Context) {
	taskID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		Mode string `json:"mode"`
	}
	_ = c.ShouldBindJSON(&input)
	triggeredBy := ""
	if subject, exists := c.Get(contextUserSubjectKey); exists {
		triggeredBy = strings.TrimSpace(fmt.Sprintf("%v", subject))
	}
	if triggeredBy == "" {
		triggeredBy = "manual"
	}
	detail, err := h.service.StartByTask(c.Request.Context(), taskID, input.Mode, triggeredBy)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "backup_verify", "manual_run", "backup_task", fmt.Sprintf("%d", taskID), "",
		fmt.Sprintf("手动触发验证（任务 ID: %d, 验证记录 ID: %d, 模式: %s）", taskID, detail.ID, detail.Mode))
	response.Success(c, detail)
}

// TriggerByRecord 基于指定备份记录触发验证（允许验证历史备份）。
func (h *VerificationHandler) TriggerByRecord(c *gin.Context) {
	recordID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input struct {
		Mode string `json:"mode"`
	}
	_ = c.ShouldBindJSON(&input)
	triggeredBy := ""
	if subject, exists := c.Get(contextUserSubjectKey); exists {
		triggeredBy = strings.TrimSpace(fmt.Sprintf("%v", subject))
	}
	if triggeredBy == "" {
		triggeredBy = "manual"
	}
	detail, err := h.service.Start(c.Request.Context(), recordID, input.Mode, triggeredBy)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "backup_verify", "manual_run", "backup_record", fmt.Sprintf("%d", recordID), "",
		fmt.Sprintf("手动触发验证（备份记录 ID: %d, 验证记录 ID: %d, 模式: %s）", recordID, detail.ID, detail.Mode))
	response.Success(c, detail)
}

func (h *VerificationHandler) List(c *gin.Context) {
	filter, err := buildVerifyFilter(c)
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

func (h *VerificationHandler) Get(c *gin.Context) {
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

func (h *VerificationHandler) StreamLogs(c *gin.Context) {
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
		response.Error(c, apperror.Internal("VERIFY_STREAM_UNSUPPORTED", "当前连接不支持日志流", nil))
		return
	}
	for _, event := range events {
		if err := writeVerifySSEEvent(c.Writer, event); err != nil {
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
			if err := writeVerifySSEEvent(c.Writer, event); err != nil {
				return
			}
			flusher.Flush()
			if event.Completed {
				return
			}
		}
	}
}

func buildVerifyFilter(c *gin.Context) (service.VerificationRecordListInput, error) {
	var filter service.VerificationRecordListInput
	if value := strings.TrimSpace(c.Query("taskId")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return filter, apperror.BadRequest("VERIFY_RECORD_FILTER_INVALID", "taskId 不合法", err)
		}
		v := uint(parsed)
		filter.TaskID = &v
	}
	if value := strings.TrimSpace(c.Query("backupRecordId")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return filter, apperror.BadRequest("VERIFY_RECORD_FILTER_INVALID", "backupRecordId 不合法", err)
		}
		v := uint(parsed)
		filter.BackupRecordID = &v
	}
	filter.Status = strings.TrimSpace(c.Query("status"))
	if dateFrom := strings.TrimSpace(c.Query("dateFrom")); dateFrom != "" {
		parsed, err := time.Parse(time.RFC3339, dateFrom)
		if err != nil {
			return filter, apperror.BadRequest("VERIFY_RECORD_FILTER_INVALID", "dateFrom 必须为 RFC3339 时间格式", err)
		}
		filter.DateFrom = &parsed
	}
	if dateTo := strings.TrimSpace(c.Query("dateTo")); dateTo != "" {
		parsed, err := time.Parse(time.RFC3339, dateTo)
		if err != nil {
			return filter, apperror.BadRequest("VERIFY_RECORD_FILTER_INVALID", "dateTo 必须为 RFC3339 时间格式", err)
		}
		filter.DateTo = &parsed
	}
	return filter, nil
}

func writeVerifySSEEvent(writer io.Writer, event backup.LogEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "event: log\ndata: %s\n\n", payload)
	return err
}
