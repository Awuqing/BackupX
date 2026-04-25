package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/security"
)

func (s *AuthService) ListTrustedDevices(ctx context.Context, subject string) ([]TrustedDeviceOutput, error) {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	devices, err := parseTrustedDevices(user.TrustedDevices)
	if err != nil {
		return nil, apperror.Internal("AUTH_TRUSTED_DEVICE_INVALID", "可信设备配置异常", err)
	}
	now := time.Now().UTC()
	output := make([]TrustedDeviceOutput, 0, len(devices))
	for _, device := range devices {
		if device.ExpiresAt.Before(now) {
			continue
		}
		output = append(output, toTrustedDeviceOutput(device))
	}
	return output, nil
}

type TrustedDeviceRevokeInput struct {
	CurrentPassword string `json:"currentPassword" binding:"required,min=8,max=128"`
}

func (s *AuthService) RevokeTrustedDevice(ctx context.Context, subject string, id string, input TrustedDeviceRevokeInput) error {
	user, err := s.userBySubject(ctx, subject)
	if err != nil {
		return err
	}
	if err := security.ComparePassword(user.PasswordHash, input.CurrentPassword); err != nil {
		return apperror.BadRequest("AUTH_WRONG_PASSWORD", "当前密码不正确", err)
	}
	devices, err := parseTrustedDevices(user.TrustedDevices)
	if err != nil {
		return apperror.Internal("AUTH_TRUSTED_DEVICE_INVALID", "可信设备配置异常", err)
	}
	found := false
	filtered := make([]TrustedDeviceRecord, 0, len(devices))
	for _, device := range devices {
		if device.ID == strings.TrimSpace(id) {
			found = true
		} else {
			filtered = append(filtered, device)
		}
	}
	if !found {
		return apperror.New(404, "AUTH_TRUSTED_DEVICE_NOT_FOUND", "可信设备不存在", nil)
	}
	encoded, err := encodeTrustedDevices(filtered)
	if err != nil {
		return apperror.Internal("AUTH_TRUSTED_DEVICE_INVALID", "可信设备配置异常", err)
	}
	user.TrustedDevices = encoded
	if err := s.users.Update(ctx, user); err != nil {
		return apperror.Internal("AUTH_TRUSTED_DEVICE_REVOKE_FAILED", "无法移除可信设备", err)
	}
	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "trusted_device_revoke",
			TargetType: "trusted_device", TargetID: strings.TrimSpace(id),
			Detail: "移除可信设备",
		})
	}
	return nil
}

func (s *AuthService) verifyTrustedDevice(ctx context.Context, user *model.User, token string, clientKey string) (bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return false, nil
	}
	devices, err := parseTrustedDevices(user.TrustedDevices)
	if err != nil {
		return false, apperror.Internal("AUTH_TRUSTED_DEVICE_INVALID", "可信设备配置异常", err)
	}
	now := time.Now().UTC()
	hash := trustedDeviceTokenHash(token)
	changed := false
	for i := range devices {
		device := &devices[i]
		if device.ExpiresAt.Before(now) {
			changed = true
			continue
		}
		if subtle.ConstantTimeCompare([]byte(device.TokenHash), []byte(hash)) != 1 {
			continue
		}
		device.LastUsedAt = now
		device.LastIP = clientKey
		changed = true
		encoded, err := encodeTrustedDevices(filterActiveTrustedDevices(devices, now))
		if err != nil {
			return false, apperror.Internal("AUTH_TRUSTED_DEVICE_INVALID", "可信设备配置异常", err)
		}
		user.TrustedDevices = encoded
		if err := s.users.Update(ctx, user); err != nil {
			return false, apperror.Internal("AUTH_TRUSTED_DEVICE_UPDATE_FAILED", "无法更新可信设备", err)
		}
		if s.auditService != nil {
			s.auditService.Record(AuditEntry{
				UserID: user.ID, Username: user.Username,
				Category: "auth", Action: "trusted_device_used",
				TargetType: "trusted_device", TargetID: device.ID, TargetName: device.Name,
				Detail: "使用可信设备跳过多因素验证", ClientIP: clientKey,
			})
		}
		return true, nil
	}
	if changed {
		encoded, err := encodeTrustedDevices(filterActiveTrustedDevices(devices, now))
		if err != nil {
			return false, apperror.Internal("AUTH_TRUSTED_DEVICE_INVALID", "可信设备配置异常", err)
		}
		user.TrustedDevices = encoded
		if err := s.users.Update(ctx, user); err != nil {
			return false, apperror.Internal("AUTH_TRUSTED_DEVICE_UPDATE_FAILED", "无法更新可信设备", err)
		}
	}
	return false, nil
}

func (s *AuthService) issueTrustedDevice(ctx context.Context, user *model.User, name string, clientKey string) (string, *TrustedDeviceOutput, error) {
	token, err := randomURLToken(32)
	if err != nil {
		return "", nil, apperror.Internal("AUTH_TRUSTED_DEVICE_CREATE_FAILED", "无法生成可信设备令牌", err)
	}
	id, err := randomURLToken(16)
	if err != nil {
		return "", nil, apperror.Internal("AUTH_TRUSTED_DEVICE_CREATE_FAILED", "无法生成可信设备编号", err)
	}
	now := time.Now().UTC()
	deviceName := normalizeTrustedDeviceName(name)
	device := TrustedDeviceRecord{
		ID:         id,
		Name:       deviceName,
		TokenHash:  trustedDeviceTokenHash(token),
		CreatedAt:  now,
		LastUsedAt: now,
		ExpiresAt:  now.Add(trustedDeviceTTL),
		LastIP:     clientKey,
	}
	devices, err := parseTrustedDevices(user.TrustedDevices)
	if err != nil {
		return "", nil, apperror.Internal("AUTH_TRUSTED_DEVICE_INVALID", "可信设备配置异常", err)
	}
	devices = append(filterActiveTrustedDevices(devices, now), device)
	if len(devices) > maxTrustedDevices {
		devices = devices[len(devices)-maxTrustedDevices:]
	}
	encoded, err := encodeTrustedDevices(devices)
	if err != nil {
		return "", nil, apperror.Internal("AUTH_TRUSTED_DEVICE_INVALID", "可信设备配置异常", err)
	}
	user.TrustedDevices = encoded
	if err := s.users.Update(ctx, user); err != nil {
		return "", nil, apperror.Internal("AUTH_TRUSTED_DEVICE_CREATE_FAILED", "无法保存可信设备", err)
	}
	output := toTrustedDeviceOutput(device)
	if s.auditService != nil {
		s.auditService.Record(AuditEntry{
			UserID: user.ID, Username: user.Username,
			Category: "auth", Action: "trusted_device_create",
			TargetType: "trusted_device", TargetID: device.ID, TargetName: device.Name,
			Detail: fmt.Sprintf("添加可信设备，有效期至 %s", device.ExpiresAt.Format(time.RFC3339)), ClientIP: clientKey,
		})
	}
	return token, &output, nil
}

func filterActiveTrustedDevices(devices []TrustedDeviceRecord, now time.Time) []TrustedDeviceRecord {
	active := make([]TrustedDeviceRecord, 0, len(devices))
	for _, device := range devices {
		if device.ExpiresAt.After(now) {
			active = append(active, device)
		}
	}
	return active
}

func trustedDeviceTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomURLToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func normalizeTrustedDeviceName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "当前设备"
	}
	if len([]rune(trimmed)) <= maxTrustedDeviceName {
		return trimmed
	}
	runes := []rune(trimmed)
	return string(runes[:maxTrustedDeviceName])
}
