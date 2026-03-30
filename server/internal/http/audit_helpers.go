package http

import (
	"fmt"

	"backupx/server/internal/service"
	"github.com/gin-gonic/gin"
)

// recordAudit 从 gin context 中提取用户信息并记录审计日志（nil 安全）
func recordAudit(c *gin.Context, auditService *service.AuditService, category, action, targetType, targetID, targetName, detail string) {
	if auditService == nil {
		return
	}
	username := ""
	if subject, exists := c.Get(contextUserSubjectKey); exists {
		username = fmt.Sprintf("%v", subject)
	}
	auditService.Record(service.AuditEntry{
		Username:   username,
		Category:   category,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		TargetName: targetName,
		Detail:     detail,
		ClientIP:   c.ClientIP(),
	})
}
