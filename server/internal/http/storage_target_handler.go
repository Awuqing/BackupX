package http

import (
	"fmt"
	"strconv"
	"strings"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type StorageTargetHandler struct {
	service *service.StorageTargetService
}

type storageTargetGoogleDriveAuthRequest struct {
	TargetID     *uint          `json:"targetId"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Description  string         `json:"description"`
	Enabled      bool           `json:"enabled"`
	Config       map[string]any `json:"config"`
	ClientID     string         `json:"clientId"`
	ClientSecret string         `json:"clientSecret"`
	FolderID     string         `json:"folderId"`
}

func NewStorageTargetHandler(service *service.StorageTargetService) *StorageTargetHandler {
	return &StorageTargetHandler{service: service}
}

func (h *StorageTargetHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, items)
}

func (h *StorageTargetHandler) Get(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, item)
}

func (h *StorageTargetHandler) Create(c *gin.Context) {
	var input service.StorageTargetUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("STORAGE_TARGET_INVALID", "存储目标参数不合法", err))
		return
	}
	item, err := h.service.Create(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, item)
}

func (h *StorageTargetHandler) Update(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var input service.StorageTargetUpsertInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("STORAGE_TARGET_INVALID", "存储目标参数不合法", err))
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, item)
}

func (h *StorageTargetHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *StorageTargetHandler) TestConnection(c *gin.Context) {
	var payload service.StorageTargetUpsertInput
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, apperror.BadRequest("STORAGE_TARGET_TEST_INVALID", "测试连接参数不合法", err))
		return
	}
	if err := h.service.TestConnection(c.Request.Context(), service.StorageTargetTestInput{Payload: payload}); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"success": true, "message": "连接成功"})
}

func (h *StorageTargetHandler) TestSavedConnection(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.TestConnection(c.Request.Context(), service.StorageTargetTestInput{TargetID: &id}); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"success": true, "message": "连接成功"})
}

func (h *StorageTargetHandler) StartGoogleDriveOAuth(c *gin.Context) {
	var request storageTargetGoogleDriveAuthRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, apperror.BadRequest("STORAGE_GOOGLE_OAUTH_INVALID", "Google Drive 授权参数不合法", err))
		return
	}
	input := service.GoogleDriveAuthStartInput{
		TargetID:     request.TargetID,
		Name:         strings.TrimSpace(request.Name),
		Description:  strings.TrimSpace(request.Description),
		Enabled:      request.Enabled,
		ClientID:     firstNonEmpty(asString(request.Config["clientId"]), request.ClientID),
		ClientSecret: firstNonEmpty(asString(request.Config["clientSecret"]), request.ClientSecret),
		FolderID:     firstNonEmpty(asString(request.Config["folderId"]), request.FolderID),
	}
	result, err := h.service.StartGoogleDriveOAuth(c.Request.Context(), input, requestOrigin(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"authUrl": result.AuthorizationURL})
}

func (h *StorageTargetHandler) CompleteGoogleDriveOAuth(c *gin.Context) {
	var input service.GoogleDriveAuthCompleteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("STORAGE_GOOGLE_OAUTH_INVALID", "Google Drive 回调参数不合法", err))
		return
	}
	item, err := h.service.CompleteGoogleDriveOAuth(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, item)
}

func (h *StorageTargetHandler) HandleGoogleDriveCallback(c *gin.Context) {
	if queryError := strings.TrimSpace(c.Query("error")); queryError != "" {
		response.Success(c, gin.H{"success": false, "message": queryError})
		return
	}
	input := service.GoogleDriveAuthCompleteInput{State: strings.TrimSpace(c.Query("state")), Code: strings.TrimSpace(c.Query("code"))}
	if input.State == "" || input.Code == "" {
		response.Error(c, apperror.BadRequest("STORAGE_GOOGLE_OAUTH_INVALID", "Google Drive 回调参数不合法", nil))
		return
	}
	item, err := h.service.CompleteGoogleDriveOAuth(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"success": true, "message": "Google Drive 授权成功", "target": item})
}

func (h *StorageTargetHandler) GoogleDriveProfile(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	profile, err := h.service.GoogleDriveProfile(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, profile)
}

func parseUintParam(c *gin.Context, key string) (uint, bool) {
	value := strings.TrimSpace(c.Param(key))
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		response.Error(c, apperror.BadRequest("INVALID_ID", fmt.Sprintf("参数 %s 不合法", key), err))
		return 0, false
	}
	return uint(parsed), true
}

func requestOrigin(c *gin.Context) string {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin != "" {
		return origin
	}
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

func asString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (h *StorageTargetHandler) GetUsage(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	usage, err := h.service.GetUsage(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, usage)
}
