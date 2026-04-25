package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/security"
)

type WebAuthnRequestContext struct {
	RPID   string
	Origin string
}

type WebAuthnRegistrationOptionsInput struct {
	CurrentPassword string `json:"currentPassword" binding:"required,min=8,max=128"`
}

type WebAuthnRegistrationFinishInput struct {
	Name       string                                `json:"name" binding:"omitempty,max=128"`
	Credential security.WebAuthnRegistrationResponse `json:"credential" binding:"required"`
}

type WebAuthnCredentialDeleteInput struct {
	CurrentPassword string `json:"currentPassword" binding:"required,min=8,max=128"`
}

type WebAuthnLoginOptionsInput struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

type webAuthnPublicKeyCredentialParam struct {
	Type string `json:"type"`
	Alg  int    `json:"alg"`
}

type webAuthnRelyingParty struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type webAuthnUserEntity struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type webAuthnCredentialDescriptor struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type webAuthnAuthenticatorSelection struct {
	UserVerification string `json:"userVerification"`
}

type WebAuthnRegistrationOptions struct {
	Challenge              string                             `json:"challenge"`
	RP                     webAuthnRelyingParty               `json:"rp"`
	User                   webAuthnUserEntity                 `json:"user"`
	PubKeyCredParams       []webAuthnPublicKeyCredentialParam `json:"pubKeyCredParams"`
	Timeout                int                                `json:"timeout"`
	Attestation            string                             `json:"attestation"`
	AuthenticatorSelection webAuthnAuthenticatorSelection     `json:"authenticatorSelection"`
	ExcludeCredentials     []webAuthnCredentialDescriptor     `json:"excludeCredentials"`
}

type WebAuthnLoginOptions struct {
	Challenge        string                         `json:"challenge"`
	RPID             string                         `json:"rpId"`
	Timeout          int                            `json:"timeout"`
	UserVerification string                         `json:"userVerification"`
	AllowCredentials []webAuthnCredentialDescriptor `json:"allowCredentials"`
}

func (s *AuthService) BeginWebAuthnRegistration(ctx context.Context, subject string, input WebAuthnRegistrationOptionsInput, request WebAuthnRequestContext) (*WebAuthnRegistrationOptions, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	if err := security.ComparePassword(user.PasswordHash, input.CurrentPassword); err != nil {
		return nil, apperror.BadRequest("AUTH_WRONG_PASSWORD", "当前密码不正确", err)
	}
	credentials, err := parseWebAuthnCredentials(user.WebAuthnCredentials)
	if err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_INVALID", "通行密钥配置异常", err)
	}
	challenge, err := security.GenerateWebAuthnChallenge()
	if err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_CHALLENGE_FAILED", "无法生成通行密钥挑战", err)
	}
	state := webAuthnChallengeState{
		Type:      "register",
		Challenge: challenge,
		RPID:      request.RPID,
		Origin:    request.Origin,
		ExpiresAt: time.Now().UTC().Add(mfaChallengeTTL),
	}
	if err := s.saveWebAuthnChallenge(ctx, user, state); err != nil {
		return nil, err
	}
	exclude := make([]webAuthnCredentialDescriptor, 0, len(credentials))
	for _, credential := range credentials {
		exclude = append(exclude, webAuthnCredentialDescriptor{Type: "public-key", ID: credential.CredentialID})
	}
	return &WebAuthnRegistrationOptions{
		Challenge: challenge,
		RP:        webAuthnRelyingParty{Name: "BackupX", ID: request.RPID},
		User: webAuthnUserEntity{
			ID:          security.EncodeBase64URL([]byte(fmt.Sprintf("%d", user.ID))),
			Name:        user.Username,
			DisplayName: user.DisplayName,
		},
		PubKeyCredParams: []webAuthnPublicKeyCredentialParam{
			{Type: "public-key", Alg: -7},
		},
		Timeout:                int(mfaChallengeTTL / time.Millisecond),
		Attestation:            "none",
		AuthenticatorSelection: webAuthnAuthenticatorSelection{UserVerification: "preferred"},
		ExcludeCredentials:     exclude,
	}, nil
}

func (s *AuthService) FinishWebAuthnRegistration(ctx context.Context, subject string, input WebAuthnRegistrationFinishInput) (*UserOutput, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	state, err := s.loadWebAuthnChallenge(user, "register")
	if err != nil {
		return nil, err
	}
	parsed, err := security.VerifyWebAuthnRegistration(input.Credential, state.Challenge, state.RPID, state.Origin)
	if err != nil {
		return nil, apperror.BadRequest("AUTH_WEBAUTHN_VERIFY_FAILED", "通行密钥注册校验失败", err)
	}
	credentials, err := parseWebAuthnCredentials(user.WebAuthnCredentials)
	if err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_INVALID", "通行密钥配置异常", err)
	}
	for _, credential := range credentials {
		if credential.CredentialID == parsed.CredentialID {
			return nil, apperror.Conflict("AUTH_WEBAUTHN_EXISTS", "该通行密钥已注册", nil)
		}
	}
	id, err := randomURLToken(16)
	if err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_SAVE_FAILED", "无法生成通行密钥编号", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "通行密钥"
	}
	credentials = append(credentials, WebAuthnCredentialRecord{
		ID:           id,
		Name:         normalizeTrustedDeviceName(name),
		CredentialID: parsed.CredentialID,
		PublicKeyX:   parsed.PublicKeyX,
		PublicKeyY:   parsed.PublicKeyY,
		SignCount:    parsed.SignCount,
		CreatedAt:    now,
	})
	encoded, err := encodeWebAuthnCredentials(credentials)
	if err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_SAVE_FAILED", "无法保存通行密钥", err)
	}
	user.WebAuthnCredentials = encoded
	user.WebAuthnChallengeCiphertext = ""
	if err := s.users.Update(ctx, user); err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_SAVE_FAILED", "无法保存通行密钥", err)
	}
	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "webauthn_register",
			TargetType: "webauthn_credential", TargetID: id, TargetName: name,
			Detail: "注册通行密钥",
		})
	}
	return ToUserOutput(user), nil
}

func (s *AuthService) BeginWebAuthnLogin(ctx context.Context, input WebAuthnLoginOptionsInput, request WebAuthnRequestContext, clientKey string) (*WebAuthnLoginOptions, error) {
	user, err := s.verifyPasswordForMFAStart(ctx, input.Username, input.Password, clientKey)
	if err != nil {
		return nil, err
	}
	credentials, err := parseWebAuthnCredentials(user.WebAuthnCredentials)
	if err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_INVALID", "通行密钥配置异常", err)
	}
	if len(credentials) == 0 {
		return nil, apperror.BadRequest("AUTH_WEBAUTHN_NOT_ENABLED", "当前账号未注册通行密钥", nil)
	}
	challenge, err := security.GenerateWebAuthnChallenge()
	if err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_CHALLENGE_FAILED", "无法生成通行密钥挑战", err)
	}
	state := webAuthnChallengeState{
		Type:      "login",
		Challenge: challenge,
		RPID:      request.RPID,
		Origin:    request.Origin,
		ExpiresAt: time.Now().UTC().Add(mfaChallengeTTL),
	}
	if err := s.saveWebAuthnChallenge(ctx, user, state); err != nil {
		return nil, err
	}
	allowed := make([]webAuthnCredentialDescriptor, 0, len(credentials))
	for _, credential := range credentials {
		allowed = append(allowed, webAuthnCredentialDescriptor{Type: "public-key", ID: credential.CredentialID})
	}
	return &WebAuthnLoginOptions{
		Challenge:        challenge,
		RPID:             request.RPID,
		Timeout:          int(mfaChallengeTTL / time.Millisecond),
		UserVerification: "preferred",
		AllowCredentials: allowed,
	}, nil
}

func (s *AuthService) VerifyWebAuthnLogin(ctx context.Context, user *model.User, assertion security.WebAuthnLoginAssertion, clientKey string) error {
	state, err := s.loadWebAuthnChallenge(user, "login")
	if err != nil {
		return err
	}
	credentials, err := parseWebAuthnCredentials(user.WebAuthnCredentials)
	if err != nil {
		return apperror.Internal("AUTH_WEBAUTHN_INVALID", "通行密钥配置异常", err)
	}
	rawID := strings.TrimSpace(assertion.RawID)
	if rawID == "" {
		rawID = strings.TrimSpace(assertion.ID)
	}
	for i := range credentials {
		credential := &credentials[i]
		if credential.CredentialID != rawID {
			continue
		}
		nextSignCount, err := security.VerifyWebAuthnAssertion(assertion, state.Challenge, state.RPID, state.Origin, security.WebAuthnCredentialMaterial{
			CredentialID: credential.CredentialID,
			PublicKeyX:   credential.PublicKeyX,
			PublicKeyY:   credential.PublicKeyY,
			SignCount:    credential.SignCount,
		})
		if err != nil {
			return apperror.Unauthorized("AUTH_WEBAUTHN_INVALID", "通行密钥校验失败", err)
		}
		credential.SignCount = nextSignCount
		credential.LastUsedAt = time.Now().UTC().Format(time.RFC3339)
		encoded, err := encodeWebAuthnCredentials(credentials)
		if err != nil {
			return apperror.Internal("AUTH_WEBAUTHN_SAVE_FAILED", "无法更新通行密钥", err)
		}
		user.WebAuthnCredentials = encoded
		user.WebAuthnChallengeCiphertext = ""
		if err := s.users.Update(ctx, user); err != nil {
			return apperror.Internal("AUTH_WEBAUTHN_SAVE_FAILED", "无法更新通行密钥", err)
		}
		if s.auditService != nil {
			s.auditService.Record(AuditEntry{
				UserID: user.ID, Username: user.Username,
				Category: "auth", Action: "webauthn_used",
				TargetType: "webauthn_credential", TargetID: credential.ID, TargetName: credential.Name,
				Detail: "使用通行密钥完成多因素验证", ClientIP: clientKey,
			})
		}
		return nil
	}
	return apperror.Unauthorized("AUTH_WEBAUTHN_INVALID", "通行密钥不存在", nil)
}

func (s *AuthService) ListWebAuthnCredentials(ctx context.Context, subject string) ([]WebAuthnCredentialOutput, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	credentials, err := parseWebAuthnCredentials(user.WebAuthnCredentials)
	if err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_INVALID", "通行密钥配置异常", err)
	}
	output := make([]WebAuthnCredentialOutput, 0, len(credentials))
	for _, credential := range credentials {
		output = append(output, toWebAuthnCredentialOutput(credential))
	}
	return output, nil
}

func (s *AuthService) DeleteWebAuthnCredential(ctx context.Context, subject string, id string, input WebAuthnCredentialDeleteInput) (*UserOutput, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	if err := security.ComparePassword(user.PasswordHash, input.CurrentPassword); err != nil {
		return nil, apperror.BadRequest("AUTH_WRONG_PASSWORD", "当前密码不正确", err)
	}
	credentials, err := parseWebAuthnCredentials(user.WebAuthnCredentials)
	if err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_INVALID", "通行密钥配置异常", err)
	}
	found := false
	filtered := make([]WebAuthnCredentialRecord, 0, len(credentials))
	for _, credential := range credentials {
		if credential.ID == strings.TrimSpace(id) {
			found = true
		} else {
			filtered = append(filtered, credential)
		}
	}
	if !found {
		return nil, apperror.New(404, "AUTH_WEBAUTHN_NOT_FOUND", "通行密钥不存在", nil)
	}
	encoded, err := encodeWebAuthnCredentials(filtered)
	if err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_SAVE_FAILED", "无法更新通行密钥", err)
	}
	user.WebAuthnCredentials = encoded
	clearTrustedDevicesIfMFAOff(user)
	if err := s.users.Update(ctx, user); err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_DELETE_FAILED", "无法删除通行密钥", err)
	}
	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "webauthn_delete",
			TargetType: "webauthn_credential", TargetID: strings.TrimSpace(id),
			Detail: "删除通行密钥",
		})
	}
	return ToUserOutput(user), nil
}

func (s *AuthService) saveWebAuthnChallenge(ctx context.Context, user *model.User, state webAuthnChallengeState) error {
	ciphertext, err := s.twoFactorCipher.EncryptJSON(state)
	if err != nil {
		return apperror.Internal("AUTH_WEBAUTHN_CHALLENGE_FAILED", "无法保存通行密钥挑战", err)
	}
	user.WebAuthnChallengeCiphertext = ciphertext
	if err := s.users.Update(ctx, user); err != nil {
		return apperror.Internal("AUTH_WEBAUTHN_CHALLENGE_FAILED", "无法保存通行密钥挑战", err)
	}
	return nil
}

func (s *AuthService) loadWebAuthnChallenge(user *model.User, challengeType string) (*webAuthnChallengeState, error) {
	if strings.TrimSpace(user.WebAuthnChallengeCiphertext) == "" {
		return nil, apperror.BadRequest("AUTH_WEBAUTHN_CHALLENGE_MISSING", "请先发起通行密钥验证", nil)
	}
	var state webAuthnChallengeState
	if err := s.twoFactorCipher.DecryptJSON(user.WebAuthnChallengeCiphertext, &state); err != nil {
		return nil, apperror.Internal("AUTH_WEBAUTHN_CHALLENGE_INVALID", "通行密钥挑战状态异常", err)
	}
	if state.Type != challengeType {
		return nil, apperror.BadRequest("AUTH_WEBAUTHN_CHALLENGE_INVALID", "通行密钥挑战类型不匹配", nil)
	}
	if state.ExpiresAt.Before(time.Now().UTC()) {
		return nil, apperror.BadRequest("AUTH_WEBAUTHN_CHALLENGE_EXPIRED", "通行密钥挑战已过期", nil)
	}
	return &state, nil
}
