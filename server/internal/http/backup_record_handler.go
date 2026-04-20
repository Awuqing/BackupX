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

type BackupRecordHandler struct {
	service        *service.BackupRecordService
	restoreService *service.RestoreService
	auditService   *service.AuditService
}

func NewBackupRecordHandler(recordService *service.BackupRecordService, restoreService *service.RestoreService, auditService *service.AuditService) *BackupRecordHandler {
	return &BackupRecordHandler{service: recordService, restoreService: restoreService, auditService: auditService}
}

func (h *BackupRecordHandler) List(c *gin.Context) {
	filter, err := buildRecordFilter(c)
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

func (h *BackupRecordHandler) Get(c *gin.Context) {
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

func (h *BackupRecordHandler) StreamLogs(c *gin.Context) {
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
		response.Error(c, apperror.Internal("BACKUP_RECORD_STREAM_UNSUPPORTED", "当前连接不支持日志流", nil))
		return
	}
	for _, event := range events {
		if err := writeSSEEvent(c.Writer, event); err != nil {
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
			if err := writeSSEEvent(c.Writer, event); err != nil {
				return
			}
			flusher.Flush()
			if event.Completed {
				return
			}
		}
	}
}

func (h *BackupRecordHandler) Download(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	result, err := h.service.Download(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	defer result.Reader.Close()
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.FileName))
	c.Header("Content-Type", "application/octet-stream")
	_, _ = io.Copy(c.Writer, result.Reader)
}

// Restore 启动一次异步恢复并返回 restoreRecordId；实际执行路由由 RestoreService
// 根据 task.NodeID 决定（本地 Master or 远程 Agent）。
func (h *BackupRecordHandler) Restore(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if h.restoreService == nil {
		response.Error(c, apperror.Internal("RESTORE_SERVICE_DISABLED", "恢复服务未启用", nil))
		return
	}
	triggeredBy := ""
	if subject, exists := c.Get(contextUserSubjectKey); exists {
		triggeredBy = strings.TrimSpace(fmt.Sprintf("%v", subject))
	}
	detail, err := h.restoreService.Start(c.Request.Context(), id, triggeredBy)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "backup_record", "restore", "backup_record", fmt.Sprintf("%d", id), "",
		fmt.Sprintf("启动恢复 (备份记录 ID: %d, 恢复记录 ID: %d)", id, detail.ID))
	response.Success(c, detail)
}

func (h *BackupRecordHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "backup_record", "delete", "backup_record", fmt.Sprintf("%d", id), "",
		fmt.Sprintf("删除备份记录 (ID: %d)", id))
	response.Success(c, gin.H{"deleted": true})
}

func (h *BackupRecordHandler) BatchDelete(c *gin.Context) {
	var input struct {
		IDs []uint `json:"ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("BACKUP_RECORD_BATCH_INVALID", "批量删除参数不合法", err))
		return
	}
	deleted := 0
	for _, id := range input.IDs {
		if err := h.service.Delete(c.Request.Context(), id); err == nil {
			deleted++
		}
	}
	recordAudit(c, h.auditService, "backup_record", "batch_delete", "backup_record", "", "", fmt.Sprintf("批量删除 %d 条备份记录", deleted))
	response.Success(c, gin.H{"deleted": deleted})
}

func buildRecordFilter(c *gin.Context) (service.BackupRecordListInput, error) {
	var filter service.BackupRecordListInput
	if taskIDValue := strings.TrimSpace(c.Query("taskId")); taskIDValue != "" {
		parsed, ok := parseUintString(taskIDValue)
		if !ok {
			return filter, apperror.BadRequest("BACKUP_RECORD_FILTER_INVALID", "taskId 不合法", nil)
		}
		filter.TaskID = &parsed
	}
	filter.Status = strings.TrimSpace(c.Query("status"))
	if dateFrom := strings.TrimSpace(c.Query("dateFrom")); dateFrom != "" {
		parsed, err := time.Parse(time.RFC3339, dateFrom)
		if err != nil {
			return filter, apperror.BadRequest("BACKUP_RECORD_FILTER_INVALID", "dateFrom 必须为 RFC3339 时间格式", err)
		}
		filter.DateFrom = &parsed
	}
	if dateTo := strings.TrimSpace(c.Query("dateTo")); dateTo != "" {
		parsed, err := time.Parse(time.RFC3339, dateTo)
		if err != nil {
			return filter, apperror.BadRequest("BACKUP_RECORD_FILTER_INVALID", "dateTo 必须为 RFC3339 时间格式", err)
		}
		filter.DateTo = &parsed
	}
	return filter, nil
}

func writeSSEEvent(writer io.Writer, event backup.LogEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "event: log\ndata: %s\n\n", payload)
	return err
}

func parseUintString(value string) (uint, bool) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(parsed), true
}
