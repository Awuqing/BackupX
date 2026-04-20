package http

import (
	"encoding/json"
	"fmt"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

// TaskExportHandler 提供任务配置 JSON 导入/导出。
type TaskExportHandler struct {
	service      *service.TaskExportService
	auditService *service.AuditService
}

func NewTaskExportHandler(s *service.TaskExportService, audit *service.AuditService) *TaskExportHandler {
	return &TaskExportHandler{service: s, auditService: audit}
}

// Export GET /api/backup/tasks/export?ids=1,2,3
// 无 ids 参数时导出全部任务。返回 application/json + Content-Disposition。
func (h *TaskExportHandler) Export(c *gin.Context) {
	var taskIDs []uint
	if v := strings.TrimSpace(c.Query("ids")); v != "" {
		for _, part := range strings.Split(v, ",") {
			if id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64); err == nil {
				taskIDs = append(taskIDs, uint(id))
			}
		}
	}
	payload, err := h.service.Export(c.Request.Context(), taskIDs)
	if err != nil {
		response.Error(c, err)
		return
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		response.Error(c, apperror.Internal("TASK_EXPORT_MARSHAL_FAILED", "无法序列化导出内容", err))
		return
	}
	filename := fmt.Sprintf("backupx-tasks-%s.json", time.Now().UTC().Format("20060102-150405"))
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = c.Writer.Write(data)
	recordAudit(c, h.auditService, "backup_task", "export", "backup_task", "", "",
		fmt.Sprintf("导出 %d 个任务的配置为 JSON", payload.TaskCount))
}

// Import POST /api/backup/tasks/import
// Body: ExportPayload JSON。返回每个任务的创建/跳过结果。
func (h *TaskExportHandler) Import(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, apperror.BadRequest("TASK_IMPORT_INVALID", "无法读取请求体", err))
		return
	}
	if len(body) == 0 {
		response.Error(c, apperror.BadRequest("TASK_IMPORT_INVALID", "请求体为空", nil))
		return
	}
	if len(body) > 1024*1024 { // 1MB 上限
		c.Writer.WriteHeader(stdhttp.StatusRequestEntityTooLarge)
		response.Error(c, apperror.BadRequest("TASK_IMPORT_TOO_LARGE", "导入文件过大（上限 1MB）", nil))
		return
	}
	var payload service.ExportPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		response.Error(c, apperror.BadRequest("TASK_IMPORT_INVALID", "JSON 格式不合法", err))
		return
	}
	if len(payload.Tasks) == 0 {
		response.Error(c, apperror.BadRequest("TASK_IMPORT_INVALID", "文件中未包含任何任务", nil))
		return
	}
	results, err := h.service.Import(c.Request.Context(), payload)
	if err != nil {
		response.Error(c, err)
		return
	}
	succ := 0
	skipped := 0
	for _, r := range results {
		if r.Success && !r.Skipped {
			succ++
		} else if r.Skipped {
			skipped++
		}
	}
	recordAudit(c, h.auditService, "backup_task", "import", "backup_task", "", "",
		fmt.Sprintf("从 JSON 导入任务：创建 %d / 跳过 %d / 失败 %d", succ, skipped, len(results)-succ-skipped))
	response.Success(c, results)
}
