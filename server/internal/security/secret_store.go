//go:build ignore

package security

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"backupx/server/internal/config"
)

type PersistedSecrets struct {
	JWTSecret     string `json:"jwtSecret"`
	EncryptionKey string `json:"encryptionKey"`
}

func EnsureSecrets(cfg *config.Config) error {
	if cfg.Security.JWTSecret != "" && cfg.Security.EncryptionKey != "" {
		return nil
	}

	storePath := filepath.Join(filepath.Dir(cfg.Database.Path), "backupx.secrets.json")
	current, err := loadSecrets(storePath)
	if err != nil {
		return err
	}
	if current == nil {
		current = &PersistedSecrets{}
	}
	if current.JWTSecret == "" {
		current.JWTSecret, err = randomHex(32)
		if err != nil {
			return err
		}
	}
	if current.EncryptionKey == "" {
		current.EncryptionKey, err = randomHex(32)
		if err != nil {
			return err
		}
	}
	if err := saveSecrets(storePath, current); err != nil {
		return err
	}
	if cfg.Security.JWTSecret == "" {
		cfg.Security.JWTSecret = current.JWTSecret
	}
	if cfg.Security.EncryptionKey == "" {
		cfg.Security.EncryptionKey = current.EncryptionKey
	}
	return nil
}

func loadSecrets(path string) (*PersistedSecrets, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read secrets: %w", err)
	}
	var secrets PersistedSecrets
	if err := json.Unmarshal(content, &secrets); err != nil {
		return nil, fmt.Errorf("decode secrets: %w", err)
	}
	return &secrets, nil
}

func saveSecrets(path string, secrets *PersistedSecrets) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create secrets dir: %w", err)
	}
	content, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return fmt.Errorf("encode secrets: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write secrets: %w", err)
	}
	return nil
}

func randomHex(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
