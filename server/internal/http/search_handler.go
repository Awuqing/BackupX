package http

import (
	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

// SearchHandler 全局搜索。
type SearchHandler struct {
	service *service.SearchService
}

func NewSearchHandler(s *service.SearchService) *SearchHandler {
	return &SearchHandler{service: s}
}

// Search GET /search?q=关键字
func (h *SearchHandler) Search(c *gin.Context) {
	query := c.Query("q")
	result, err := h.service.Search(c.Request.Context(), query)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, result)
}
