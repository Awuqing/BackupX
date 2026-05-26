package http

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type ReportHandler struct {
	service *service.ReportService
}

func NewReportHandler(reportService *service.ReportService) *ReportHandler {
	return &ReportHandler{service: reportService}
}

func reportDays(c *gin.Context) int {
	days := 30
	if v := strings.TrimSpace(c.Query("days")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			days = parsed
		}
	}
	return days
}

// Compliance 返回 JSON 合规报表（按任务的备份合规证据 + 汇总）。
func (h *ReportHandler) Compliance(c *gin.Context) {
	payload, err := h.service.ComplianceReport(c.Request.Context(), reportDays(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

// ComplianceCSV 把合规报表导出为 CSV（供审计归档）。带 UTF-8 BOM 以便 Excel 正确识别中文。
func (h *ReportHandler) ComplianceCSV(c *gin.Context) {
	report, err := h.service.ComplianceReport(c.Request.Context(), reportDays(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	filename := fmt.Sprintf("backupx-compliance-%s.csv", report.GeneratedAt.Format("20060102-150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	_, _ = c.Writer.WriteString("\ufeff") // UTF-8 BOM
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{
		"任务ID", "任务名", "类型", "启用", "节点", "加密", "保留天数", "SLA(RPO小时)",
		"周期内运行", "成功", "失败", "成功率", "最近状态", "最近运行(UTC)", "最近成功(UTC)", "保护字节数", "合规判定",
	})
	for _, row := range report.Tasks {
		_ = w.Write([]string{
			strconv.FormatUint(uint64(row.TaskID), 10),
			row.TaskName,
			row.Type,
			boolCN(row.Enabled),
			row.NodeName,
			boolCN(row.Encrypted),
			strconv.Itoa(row.RetentionDays),
			strconv.Itoa(row.SLAHoursRPO),
			strconv.Itoa(row.TotalRuns),
			strconv.Itoa(row.Successes),
			strconv.Itoa(row.Failures),
			fmt.Sprintf("%.2f%%", row.SuccessRate*100),
			row.LastStatus,
			fmtTimePtr(row.LastRunAt),
			fmtTimePtr(row.LastSuccessAt),
			strconv.FormatInt(row.ProtectedBytes, 10),
			row.Risk,
		})
	}
	w.Flush()
}

func boolCN(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}
