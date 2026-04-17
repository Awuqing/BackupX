package http

import (
	stdhttp "net/http"
	"strconv"
	"strings"

	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)

// AgentHandler 实现 Agent 调用 Master 的 HTTP API。
// 全部端点通过 X-Agent-Token 头做节点认证，不使用 JWT。
type AgentHandler struct {
	agentService *service.AgentService
	nodeService  *service.NodeService
}

func NewAgentHandler(agentService *service.AgentService, nodeService *service.NodeService) *AgentHandler {
	return &AgentHandler{agentService: agentService, nodeService: nodeService}
}

// extractToken 从请求头或 JSON body 中提取 Agent Token。
func extractToken(c *gin.Context) string {
	if t := strings.TrimSpace(c.GetHeader("X-Agent-Token")); t != "" {
		return t
	}
	// Authorization: Bearer <token>
	if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

// Heartbeat 扩展原有 heartbeat：除上报状态外，返回节点 ID 给 Agent 做后续调用。
func (h *AgentHandler) Heartbeat(c *gin.Context) {
	var input struct {
		Token        string `json:"token"`
		Hostname     string `json:"hostname"`
		IPAddress    string `json:"ipAddress"`
		AgentVersion string `json:"agentVersion"`
		OS           string `json:"os"`
		Arch         string `json:"arch"`
	}
	_ = c.ShouldBindJSON(&input)
	// token 优先走 body（向后兼容），否则从 header 读
	token := input.Token
	if token == "" {
		token = extractToken(c)
	}
	if token == "" {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": "missing token"})
		return
	}
	if err := h.nodeService.Heartbeat(c.Request.Context(), token, input.Hostname, input.IPAddress, input.AgentVersion, input.OS, input.Arch); err != nil {
		response.Error(c, err)
		return
	}
	// 返回节点元信息给 Agent（node_id 用于后续 API 路径）
	node, err := h.agentService.AuthenticatedNode(c.Request.Context(), token)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{
		"status": "ok",
		"nodeId": node.ID,
		"name":   node.Name,
	})
}

// Poll Agent 长轮询获取下一条待执行命令。
// 无命令时返回 {command: null}。
func (h *AgentHandler) Poll(c *gin.Context) {
	node, err := h.agentService.AuthenticatedNode(c.Request.Context(), extractToken(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	cmd, err := h.agentService.PollCommand(c.Request.Context(), node)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"command": cmd})
}

// SubmitCommandResult Agent 上报命令执行结果。
func (h *AgentHandler) SubmitCommandResult(c *gin.Context) {
	node, err := h.agentService.AuthenticatedNode(c.Request.Context(), extractToken(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	var input service.AgentCommandResult
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": err.Error()})
		return
	}
	if err := h.agentService.SubmitCommandResult(c.Request.Context(), node, uint(id), input); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}

// GetTaskSpec Agent 拉取任务规格（含解密后的存储配置）。
func (h *AgentHandler) GetTaskSpec(c *gin.Context) {
	node, err := h.agentService.AuthenticatedNode(c.Request.Context(), extractToken(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	spec, err := h.agentService.GetTaskSpec(c.Request.Context(), node, uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, spec)
}

// UpdateRecord Agent 更新备份记录（进度/完成状态/日志）。
func (h *AgentHandler) UpdateRecord(c *gin.Context) {
	node, err := h.agentService.AuthenticatedNode(c.Request.Context(), extractToken(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	var input service.AgentRecordUpdate
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": err.Error()})
		return
	}
	if err := h.agentService.UpdateRecord(c.Request.Context(), node, uint(id), input); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}
