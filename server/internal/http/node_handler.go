package http

import (
	stdhttp "net/http"
	"strconv"

	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type NodeHandler struct {
	service *service.NodeService
}

func NewNodeHandler(service *service.NodeService) *NodeHandler {
	return &NodeHandler{service: service}
}

func (h *NodeHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, items)
}

func (h *NodeHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	item, err := h.service.Get(c.Request.Context(), uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, item)
}

func (h *NodeHandler) Create(c *gin.Context) {
	var input service.NodeCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": err.Error()})
		return
	}
	token, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"token": token})
}

func (h *NodeHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context(), uint(id)); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *NodeHandler) ListDirectory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	path := c.DefaultQuery("path", "/")
	entries, err := h.service.ListDirectory(c.Request.Context(), uint(id), path)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, entries)
}

func (h *NodeHandler) Heartbeat(c *gin.Context) {
	var input struct {
		Token        string `json:"token" binding:"required"`
		Hostname     string `json:"hostname"`
		IPAddress    string `json:"ipAddress"`
		AgentVersion string `json:"agentVersion"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": err.Error()})
		return
	}
	if err := h.service.Heartbeat(c.Request.Context(), input.Token, input.Hostname, input.IPAddress, input.AgentVersion); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}
