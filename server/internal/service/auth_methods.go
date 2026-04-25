package service

import (
	"encoding/json"
	"strings"
	"time"

	"backupx/server/internal/model"
)

const (
	mfaChallengeTTL      = 5 * time.Minute
	trustedDeviceTTL     = 30 * 24 * time.Hour
	maxTrustedDeviceName = 128
	maxTrustedDevices    = 10
)

type WebAuthnCredentialRecord struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CredentialID string `json:"credentialId"`
	PublicKeyX   string `json:"publicKeyX"`
	PublicKeyY   string `json:"publicKeyY"`
	SignCount    uint32 `json:"signCount"`
	CreatedAt    string `json:"createdAt"`
	LastUsedAt   string `json:"lastUsedAt,omitempty"`
}

type WebAuthnCredentialOutput struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt,omitempty"`
}

type webAuthnChallengeState struct {
	Type      string    `json:"type"`
	Challenge string    `json:"challenge"`
	RPID      string    `json:"rpId"`
	Origin    string    `json:"origin"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type TrustedDeviceRecord struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	TokenHash  string    `json:"tokenHash"`
	CreatedAt  time.Time `json:"createdAt"`
	LastUsedAt time.Time `json:"lastUsedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	LastIP     string    `json:"lastIp"`
}

type TrustedDeviceOutput struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt"`
	ExpiresAt  string `json:"expiresAt"`
	LastIP     string `json:"lastIp"`
}

type pendingOutOfBandOTP struct {
	Channel   string    `json:"channel"`
	CodeHash  string    `json:"codeHash"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func userMFAEnabled(user *model.User) bool {
	if user == nil {
		return false
	}
	return user.TwoFactorEnabled ||
		strings.TrimSpace(user.WebAuthnCredentials) != "" ||
		user.EmailOTPEnabled ||
		user.SMSOTPEnabled
}

func clearTrustedDevicesIfMFAOff(user *model.User) {
	if user == nil || userMFAEnabled(user) {
		return
	}
	user.TrustedDevices = ""
	user.OutOfBandOTPCiphertext = ""
	user.WebAuthnChallengeCiphertext = ""
}

func parseWebAuthnCredentials(value string) ([]WebAuthnCredentialRecord, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var credentials []WebAuthnCredentialRecord
	if err := json.Unmarshal([]byte(value), &credentials); err != nil {
		return nil, err
	}
	return credentials, nil
}

func encodeWebAuthnCredentials(credentials []WebAuthnCredentialRecord) (string, error) {
	if len(credentials) == 0 {
		return "", nil
	}
	encoded, err := json.Marshal(credentials)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func webAuthnCredentialCount(user *model.User) int {
	if user == nil {
		return 0
	}
	credentials, err := parseWebAuthnCredentials(user.WebAuthnCredentials)
	if err != nil {
		return 0
	}
	return len(credentials)
}

func parseTrustedDevices(value string) ([]TrustedDeviceRecord, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var devices []TrustedDeviceRecord
	if err := json.Unmarshal([]byte(value), &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

func encodeTrustedDevices(devices []TrustedDeviceRecord) (string, error) {
	if len(devices) == 0 {
		return "", nil
	}
	encoded, err := json.Marshal(devices)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func trustedDeviceCount(user *model.User) int {
	if user == nil {
		return 0
	}
	devices, err := parseTrustedDevices(user.TrustedDevices)
	if err != nil {
		return 0
	}
	now := time.Now().UTC()
	count := 0
	for _, device := range devices {
		if device.ExpiresAt.After(now) {
			count++
		}
	}
	return count
}

func toWebAuthnCredentialOutput(record WebAuthnCredentialRecord) WebAuthnCredentialOutput {
	return WebAuthnCredentialOutput{
		ID:         record.ID,
		Name:       record.Name,
		CreatedAt:  record.CreatedAt,
		LastUsedAt: record.LastUsedAt,
	}
}

func toTrustedDeviceOutput(record TrustedDeviceRecord) TrustedDeviceOutput {
	return TrustedDeviceOutput{
		ID:         record.ID,
		Name:       record.Name,
		CreatedAt:  record.CreatedAt.Format(time.RFC3339),
		LastUsedAt: record.LastUsedAt.Format(time.RFC3339),
		ExpiresAt:  record.ExpiresAt.Format(time.RFC3339),
		LastIP:     record.LastIP,
	}
}
