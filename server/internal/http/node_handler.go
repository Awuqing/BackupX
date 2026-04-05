package http

import (
	"fmt"
	stdhttp "net/http"
	"strconv"

	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type NodeHandler struct {
	service      *service.NodeService
	auditService *service.AuditService
}

func NewNodeHandler(service *service.NodeService, auditService *service.AuditService) *NodeHandler {
	return &NodeHandler{service: service, auditService: auditService}
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
	recordAudit(c, h.auditService, "node", "create", "node", "", input.Name,
		fmt.Sprintf("创建远程节点「%s」", input.Name))
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
	recordAudit(c, h.auditService, "node", "delete", "node", fmt.Sprintf("%d", id), "",
		fmt.Sprintf("删除节点 (ID: %d)", id))
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

func (h *NodeHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	var input service.NodeUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": err.Error()})
		return
	}
	item, err := h.service.Update(c.Request.Context(), uint(id), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "node", "update", "node", fmt.Sprintf("%d", id), item.Name,
		fmt.Sprintf("更新节点「%s」(ID: %d)", item.Name, id))
	response.Success(c, item)
}

func (h *NodeHandler) Heartbeat(c *gin.Context) {
	var input struct {
		Token        string `json:"token" binding:"required"`
		Hostname     string `json:"hostname"`
		IPAddress    string `json:"ipAddress"`
		AgentVersion string `json:"agentVersion"`
		OS           string `json:"os"`
		Arch         string `json:"arch"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": err.Error()})
		return
	}
	if err := h.service.Heartbeat(c.Request.Context(), input.Token, input.Hostname, input.IPAddress, input.AgentVersion, input.OS, input.Arch); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}
