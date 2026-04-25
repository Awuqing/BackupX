package http

import (
	"net"
	"strings"

	"backupx/server/internal/apperror"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) SetupStatus(c *gin.Context) {
	initialized, err := h.authService.SetupStatus(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"initialized": initialized})
}

func (h *AuthHandler) Setup(c *gin.Context) {
	var input service.SetupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_SETUP_INVALID", "初始化参数不合法", err))
		return
	}
	payload, err := h.authService.Setup(c.Request.Context(), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_LOGIN_INVALID", "登录参数不合法", err))
		return
	}
	payload, err := h.authService.Login(c.Request.Context(), input, ClientKey(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

func (h *AuthHandler) Profile(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	user, err := h.authService.GetCurrentUser(c.Request.Context(), subject)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_PASSWORD_INVALID", "参数不合法", err))
		return
	}
	if err := h.authService.ChangePassword(c.Request.Context(), subject, input); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"changed": true})
}

func (h *AuthHandler) PrepareTwoFactor(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.TwoFactorSetupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_2FA_INVALID", "参数不合法", err))
		return
	}
	payload, err := h.authService.PrepareTwoFactor(c.Request.Context(), subject, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

func (h *AuthHandler) EnableTwoFactor(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.EnableTwoFactorInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_2FA_INVALID", "参数不合法", err))
		return
	}
	user, err := h.authService.EnableTwoFactor(c.Request.Context(), subject, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) DisableTwoFactor(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.DisableTwoFactorInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_2FA_INVALID", "参数不合法", err))
		return
	}
	user, err := h.authService.DisableTwoFactor(c.Request.Context(), subject, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) RegenerateRecoveryCodes(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.RegenerateRecoveryCodesInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_2FA_INVALID", "参数不合法", err))
		return
	}
	payload, err := h.authService.RegenerateRecoveryCodes(c.Request.Context(), subject, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, payload)
}

func (h *AuthHandler) ConfigureOTP(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.OTPConfigInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_OTP_INVALID", "参数不合法", err))
		return
	}
	user, err := h.authService.ConfigureOutOfBandOTP(c.Request.Context(), subject, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) SendLoginOTP(c *gin.Context) {
	var input service.LoginOTPInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_OTP_INVALID", "参数不合法", err))
		return
	}
	if err := h.authService.SendLoginOTP(c.Request.Context(), input, ClientKey(c)); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"sent": true})
}

func (h *AuthHandler) BeginWebAuthnRegistration(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.WebAuthnRegistrationOptionsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_WEBAUTHN_INVALID", "参数不合法", err))
		return
	}
	options, err := h.authService.BeginWebAuthnRegistration(c.Request.Context(), subject, input, webAuthnRequestContext(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, options)
}

func (h *AuthHandler) FinishWebAuthnRegistration(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.WebAuthnRegistrationFinishInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_WEBAUTHN_INVALID", "参数不合法", err))
		return
	}
	user, err := h.authService.FinishWebAuthnRegistration(c.Request.Context(), subject, input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) BeginWebAuthnLogin(c *gin.Context) {
	var input service.WebAuthnLoginOptionsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_WEBAUTHN_INVALID", "参数不合法", err))
		return
	}
	options, err := h.authService.BeginWebAuthnLogin(c.Request.Context(), input, webAuthnRequestContext(c), ClientKey(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, options)
}

func (h *AuthHandler) ListWebAuthnCredentials(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	items, err := h.authService.ListWebAuthnCredentials(c.Request.Context(), subject)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, items)
}

func (h *AuthHandler) DeleteWebAuthnCredential(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.WebAuthnCredentialDeleteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_WEBAUTHN_INVALID", "参数不合法", err))
		return
	}
	user, err := h.authService.DeleteWebAuthnCredential(c.Request.Context(), subject, c.Param("id"), input)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) ListTrustedDevices(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	items, err := h.authService.ListTrustedDevices(c.Request.Context(), subject)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, items)
}

func (h *AuthHandler) RevokeTrustedDevice(c *gin.Context) {
	subjectValue, _ := c.Get(contextUserSubjectKey)
	subject, err := service.SubjectFromContextValue(subjectValue)
	if err != nil {
		response.Error(c, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效登录态", err))
		return
	}
	var input service.TrustedDeviceRevokeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, apperror.BadRequest("AUTH_TRUSTED_DEVICE_INVALID", "参数不合法", err))
		return
	}
	if err := h.authService.RevokeTrustedDevice(c.Request.Context(), subject, c.Param("id"), input); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	response.Success(c, gin.H{"loggedOut": true})
}

func webAuthnRequestContext(c *gin.Context) service.WebAuthnRequestContext {
	host := firstForwardedValue(c.Request.Host)
	if forwardedHost := firstForwardedValue(c.GetHeader("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	rpID := host
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		rpID = parsedHost
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := firstForwardedValue(c.GetHeader("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	}
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		origin = scheme + "://" + host
	}
	return service.WebAuthnRequestContext{RPID: rpID, Origin: origin}
}

func firstForwardedValue(value string) string {
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
