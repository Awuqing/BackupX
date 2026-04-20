package http

import (
	"encoding/csv"
	"fmt"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/repository"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuditHandler struct {
	auditService *service.AuditService
}

func NewAuditHandler(auditService *service.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

// List 多字段筛选分页查询审计日志。
// 支持参数：category, action, username, targetId, keyword, dateFrom, dateTo, limit, offset。
// 向后兼容：若仅传 category + limit + offset，行为与旧版一致。
func (h *AuditHandler) List(c *gin.Context) {
	opts, err := parseAuditFilter(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	result, err := h.auditService.ListAdvanced(c.Request.Context(), opts)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, result)
}

// Export 导出 CSV。同筛选参数，最多 10000 行。
// 文件名带时间戳避免浏览器缓存覆盖。
func (h *AuditHandler) Export(c *gin.Context) {
	opts, err := parseAuditFilter(c)
	if err != nil {
		response.Error(c, err)
		return
	}
	// 导出不分页：覆盖掉 List 的默认 limit
	opts.Limit = 0
	opts.Offset = 0
	items, err := h.auditService.ExportAll(c.Request.Context(), opts)
	if err != nil {
		response.Error(c, err)
		return
	}
	filename := fmt.Sprintf("backupx-audit-%s.csv", time.Now().UTC().Format("20060102-150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	// UTF-8 BOM 让 Excel 正确识别中文
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"时间", "用户", "类别", "动作", "目标类型", "目标 ID", "目标名", "详情", "客户端 IP"})
	for _, item := range items {
		_ = writer.Write([]string{
			item.CreatedAt.UTC().Format(time.RFC3339),
			item.Username,
			item.Category,
			item.Action,
			item.TargetType,
			item.TargetID,
			item.TargetName,
			item.Detail,
			item.ClientIP,
		})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		c.Writer.WriteHeader(stdhttp.StatusInternalServerError)
	}
}

// parseAuditFilter 解析查询参数为 repository 选项。
func parseAuditFilter(c *gin.Context) (repository.AuditLogListOptions, error) {
	opts := repository.AuditLogListOptions{
		Category: strings.TrimSpace(c.Query("category")),
		Action:   strings.TrimSpace(c.Query("action")),
		Username: strings.TrimSpace(c.Query("username")),
		TargetID: strings.TrimSpace(c.Query("targetId")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
	}
	if v := strings.TrimSpace(c.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			opts.Limit = n
		}
	}
	if v := strings.TrimSpace(c.Query("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			opts.Offset = n
		}
	}
	if v := strings.TrimSpace(c.Query("dateFrom")); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return opts, apperror.BadRequest("AUDIT_FILTER_INVALID", "dateFrom 必须为 RFC3339 时间格式", err)
		}
		opts.DateFrom = &parsed
	}
	if v := strings.TrimSpace(c.Query("dateTo")); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return opts, apperror.BadRequest("AUDIT_FILTER_INVALID", "dateTo 必须为 RFC3339 时间格式", err)
		}
		opts.DateTo = &parsed
	}
	return opts, nil
}
