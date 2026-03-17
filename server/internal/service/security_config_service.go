package service

import (
	"context"
	"fmt"
	"strings"

	"backupx/server/internal/config"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
)

const (
	jwtSecretKey     = "security.jwt_secret"
	encryptionKeyKey = "security.encryption_key"
)

type ResolvedSecurity struct {
	JWTSecret     string
	EncryptionKey string
}

func ResolveSecurity(ctx context.Context, cfg config.SecurityConfig, repo repository.SystemConfigRepository) (ResolvedSecurity, error) {
	jwtSecret, err := ensureSecurityValue(ctx, repo, jwtSecretKey, cfg.JWTSecret, 48)
	if err != nil {
		return ResolvedSecurity{}, fmt.Errorf("resolve jwt secret: %w", err)
	}
	encryptionKey, err := ensureSecurityValue(ctx, repo, encryptionKeyKey, cfg.EncryptionKey, 48)
	if err != nil {
		return ResolvedSecurity{}, fmt.Errorf("resolve encryption key: %w", err)
	}
	return ResolvedSecurity{JWTSecret: jwtSecret, EncryptionKey: encryptionKey}, nil
}

func ensureSecurityValue(ctx context.Context, repo repository.SystemConfigRepository, key, configuredValue string, size int) (string, error) {
	if strings.TrimSpace(configuredValue) != "" {
		if err := repo.Upsert(ctx, &model.SystemConfig{Key: key, Value: configuredValue, Encrypted: false}); err != nil {
			return "", err
		}
		return configuredValue, nil
	}

	stored, err := repo.GetByKey(ctx, key)
	if err != nil {
		return "", err
	}
	if stored != nil && strings.TrimSpace(stored.Value) != "" {
		return stored.Value, nil
	}

	generated, err := security.GenerateSecret(size)
	if err != nil {
		return "", err
	}
	if err := repo.Upsert(ctx, &model.SystemConfig{Key: key, Value: generated, Encrypted: false}); err != nil {
		return "", err
	}
	return generated, nil
}
