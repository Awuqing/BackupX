package http

import (
	storageRclone "backupx/server/internal/storage/rclone"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

// RcloneHandler 处理 rclone 后端元数据查询。
type RcloneHandler struct{}

func NewRcloneHandler() *RcloneHandler {
	return &RcloneHandler{}
}

// ListBackends 返回所有可用的 rclone 后端及其配置选项。
func (h *RcloneHandler) ListBackends(c *gin.Context) {
	backends := storageRclone.ListBackends()
	response.Success(c, backends)
}
