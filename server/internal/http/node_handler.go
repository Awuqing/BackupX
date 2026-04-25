package http

import (
	"encoding/base64"
	"fmt"
	stdhttp "net/http"
	"strconv"

	"backupx/server/internal/apperror"
	"backupx/server/internal/installscript"
	"backupx/server/internal/repository"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type NodeHandler struct {
	service         *service.NodeService
	auditService    *service.AuditService
	installTokenSvc *service.InstallTokenService
	userRepo        repository.UserRepository
	externalURL     string
}

// NewNodeHandler 构造 handler。
// userRepo 用于把 JWT subject（用户名）解析为 user.ID，填入 install_token.created_by_id 做审计追溯；
// 传 nil 时 created_by_id 记为 0（仍可用，不阻断）。
func NewNodeHandler(
	nodeService *service.NodeService,
	auditService *service.AuditService,
	installTokenSvc *service.InstallTokenService,
	userRepo repository.UserRepository,
	externalURL string,
) *NodeHandler {
	return &NodeHandler{
		service:         nodeService,
		auditService:    auditService,
		installTokenSvc: installTokenSvc,
		userRepo:        userRepo,
		externalURL:     externalURL,
	}
}

// resolveCurrentUserID 从 JWT subject 解析出 user.ID，失败返回 0。
func (h *NodeHandler) resolveCurrentUserID(c *gin.Context) uint {
	if h.userRepo == nil {
		return 0
	}
	subjectValue, ok := c.Get(contextUserSubjectKey)
	if !ok {
		return 0
	}
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil || subject == "" {
		return 0
	}
	user, err := h.userRepo.FindByUsername(c.Request.Context(), subject)
	if err != nil || user == nil {
		return 0
	}
	return user.ID
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

// BatchCreate 批量创建远程节点。
func (h *NodeHandler) BatchCreate(c *gin.Context) {
	var input struct {
		Names []string `json:"names" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": err.Error()})
		return
	}
	results, err := h.service.BatchCreate(c.Request.Context(), input.Names)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "node", "batch_create", "node", "",
		fmt.Sprintf("%d", len(results)), fmt.Sprintf("批量创建 %d 个节点", len(results)))
	response.Success(c, results)
}

// RotateToken 轮换节点的 agent token。
func (h *NodeHandler) RotateToken(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	tok, err := h.service.RotateToken(c.Request.Context(), uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "node", "rotate_token", "node",
		fmt.Sprintf("%d", id), "",
		fmt.Sprintf("轮换节点 Token (ID: %d)", id))
	response.Success(c, gin.H{"newToken": tok})
}

// CreateInstallToken 生成一次性安装令牌。
func (h *NodeHandler) CreateInstallToken(c *gin.Context) {
	if h.installTokenSvc == nil {
		response.Error(c, apperror.New(stdhttp.StatusServiceUnavailable,
			"INSTALL_TOKEN_DISABLED", "一键部署未启用", nil))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	var input struct {
		Mode         string `json:"mode"`
		Arch         string `json:"arch"`
		AgentVersion string `json:"agentVersion"`
		DownloadSrc  string `json:"downloadSrc"`
		TTLSeconds   int    `json:"ttlSeconds"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": err.Error()})
		return
	}
	// 默认值
	if input.Mode == "" {
		input.Mode = "systemd"
	}
	if input.Arch == "" {
		input.Arch = "auto"
	}
	if input.DownloadSrc == "" {
		input.DownloadSrc = "github"
	}
	if input.TTLSeconds == 0 {
		input.TTLSeconds = 900
	}

	out, err := h.installTokenSvc.Create(c.Request.Context(), service.InstallTokenInput{
		NodeID:       uint(id),
		Mode:         input.Mode,
		Arch:         input.Arch,
		AgentVersion: input.AgentVersion,
		DownloadSrc:  input.DownloadSrc,
		TTLSeconds:   input.TTLSeconds,
		CreatedByID:  h.resolveCurrentUserID(c),
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "install_token", "create", "node",
		fmt.Sprintf("%d", id), out.Node.Name,
		fmt.Sprintf("生成 %s/%s install token TTL=%ds", input.Mode, input.Arch, input.TTLSeconds))

	masterURL := resolveMasterURL(c, h.externalURL)
	script, err := renderInstallScript(masterURL, out.Node, out.Record)
	if err != nil {
		response.Error(c, err)
		return
	}
	// 使用 /api/install/... 而非 /install/... —— 让反向代理的 /api/ 转发规则
	// 自动接管，避免 SPA fallback 把请求当成前端路由返回 index.html（issue #46）。
	// 同时返回 /install/... 备用地址，兼容会剥离 /api 前缀的外层反向代理。
	// scriptBase64 让前端可以生成不依赖公开下载路径的嵌入式命令，解决 Lucky 等代理
	// 把 /api/install/* 也 fallback 到 index.html 的场景。
	body := gin.H{
		"installToken":       out.Token,
		"expiresAt":          out.ExpiresAt,
		"url":                masterURL + "/api/install/" + out.Token,
		"fallbackUrl":        masterURL + "/install/" + out.Token,
		"scriptBase64":       base64.StdEncoding.EncodeToString([]byte(script)),
		"composeUrl":         "",
		"fallbackComposeUrl": "",
	}
	if input.Mode == "docker" {
		body["composeUrl"] = masterURL + "/api/install/" + out.Token + "/compose.yml"
		body["fallbackComposeUrl"] = masterURL + "/install/" + out.Token + "/compose.yml"
	}
	response.Success(c, body)
}

// PreviewScript 预览安装脚本（token 字段用 <AGENT_TOKEN> 占位，不消费 install token）。
// 用于 UI Step 3 展开"脚本预览"。
func (h *NodeHandler) PreviewScript(c *gin.Context) {
	mode := c.DefaultQuery("mode", "systemd")
	arch := c.DefaultQuery("arch", "auto")
	ver := c.Query("agentVersion")
	if ver == "" {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": "agentVersion required"})
		return
	}
	src := c.DefaultQuery("downloadSrc", "github")
	ctx := installscript.Context{
		MasterURL:     resolveMasterURL(c, h.externalURL),
		AgentToken:    "<AGENT_TOKEN>",
		AgentVersion:  ver,
		Mode:          mode,
		Arch:          arch,
		DownloadBase:  installscript.DownloadBaseFor(src),
		InstallPrefix: "/opt/backupx-agent",
	}
	script, err := installscript.RenderScript(ctx)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Data(stdhttp.StatusOK, "text/x-shellscript; charset=utf-8", []byte(script))
}
