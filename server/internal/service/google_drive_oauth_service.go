package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
)

type GoogleDriveOAuthResult struct {
	TargetID *uint
	Config   storage.GoogleDriveConfig
	State    string
}

type GoogleDriveOAuthService struct {
	sessions      repository.OAuthSessionRepository
	cipher        *codec.Cipher
	now           func() time.Time
	generateState func() (string, error)
	exchangeCode  func(context.Context, *oauth2.Config, string) (*oauth2.Token, error)
	ttl           time.Duration
}

type googleDriveOAuthPayload struct {
	TargetID *uint                     `json:"targetId,omitempty"`
	Config   storage.GoogleDriveConfig `json:"config"`
}

func NewGoogleDriveOAuthService(sessions repository.OAuthSessionRepository, cipher *codec.Cipher) *GoogleDriveOAuthService {
	return &GoogleDriveOAuthService{
		sessions: sessions,
		cipher:   cipher,
		now:      func() time.Time { return time.Now().UTC() },
		generateState: func() (string, error) {
			return security.GenerateSecret(24)
		},
		exchangeCode: func(ctx context.Context, config *oauth2.Config, code string) (*oauth2.Token, error) {
			return config.Exchange(ctx, code)
		},
		ttl: 10 * time.Minute,
	}
}

func (s *GoogleDriveOAuthService) Start(ctx context.Context, targetID *uint, cfg storage.GoogleDriveConfig) (string, string, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return "", "", fmt.Errorf("google drive client credentials are required")
	}
	if strings.TrimSpace(cfg.RedirectURL) == "" {
		return "", "", fmt.Errorf("google drive redirect url is required")
	}
	state, err := s.generateState()
	if err != nil {
		return "", "", fmt.Errorf("generate oauth state: %w", err)
	}
	payload := googleDriveOAuthPayload{TargetID: targetID, Config: cfg}
	ciphertext, err := s.cipher.EncryptValue(payload)
	if err != nil {
		return "", "", fmt.Errorf("encrypt oauth payload: %w", err)
	}
	now := s.now()
	session := &model.OAuthSession{ProviderType: string(storage.ProviderTypeGoogleDrive), State: state, PayloadCiphertext: ciphertext, TargetID: targetID, ExpiresAt: now.Add(s.ttl)}
	if err := s.sessions.Create(ctx, session); err != nil {
		return "", "", fmt.Errorf("create oauth session: %w", err)
	}
	oauthConfig := s.oauthConfig(cfg)
	url := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	return url, state, nil
}

func (s *GoogleDriveOAuthService) Complete(ctx context.Context, state string, code string) (*GoogleDriveOAuthResult, error) {
	session, err := s.sessions.FindByState(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("find oauth session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("google drive oauth state not found")
	}
	now := s.now()
	if session.UsedAt != nil {
		return nil, fmt.Errorf("google drive oauth state already used")
	}
	if now.After(session.ExpiresAt) {
		return nil, fmt.Errorf("google drive oauth state expired")
	}
	var payload googleDriveOAuthPayload
	if err := s.cipher.DecryptValue(session.PayloadCiphertext, &payload); err != nil {
		return nil, fmt.Errorf("decrypt oauth session payload: %w", err)
	}
	token, err := s.exchangeCode(ctx, s.oauthConfig(payload.Config), code)
	if err != nil {
		return nil, fmt.Errorf("exchange google drive oauth code: %w", err)
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		return nil, fmt.Errorf("google drive oauth response missing refresh token")
	}
	payload.Config.RefreshToken = token.RefreshToken
	session.UsedAt = &now
	if err := s.sessions.Update(ctx, session); err != nil {
		return nil, fmt.Errorf("mark oauth session used: %w", err)
	}
	return &GoogleDriveOAuthResult{TargetID: payload.TargetID, Config: payload.Config, State: state}, nil
}

func (s *GoogleDriveOAuthService) oauthConfig(cfg storage.GoogleDriveConfig) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveScope},
	}
}
