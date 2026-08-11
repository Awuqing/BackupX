package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesDefaultsWithoutConfigFile(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("expected default host, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8340 {
		t.Fatalf("expected default port, got %d", cfg.Server.Port)
	}
	if cfg.Database.Path != "./data/backupx.db" {
		t.Fatalf("expected default database path, got %s", cfg.Database.Path)
	}
	if len(cfg.Server.TrustedProxies) != 2 {
		t.Fatalf("expected loopback trusted proxies, got %#v", cfg.Server.TrustedProxies)
	}
}

func TestLoadRejectsInvalidExternalURLAndTrustedProxy(t *testing.T) {
	tests := []string{
		"server:\n  external_url: \"ssh://master.example.com\"\n",
		"server:\n  trusted_proxies: [\"not-an-ip\"]\n",
	}
	for _, content := range tests {
		configPath := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(configPath); err == nil {
			t.Fatalf("expected invalid configuration to fail: %s", content)
		}
	}
}

func TestLoadReadsServerExternalURLFromFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte("server:\n  external_url: \"https://backup.example.com\"\n")
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.ExternalURL != "https://backup.example.com" {
		t.Fatalf("expected external URL from config, got %q", cfg.Server.ExternalURL)
	}
}

func TestLoadReadsServerExternalURLFromEnv(t *testing.T) {
	t.Setenv("BACKUPX_SERVER_EXTERNAL_URL", "https://env-backup.example.com")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Server.ExternalURL != "https://env-backup.example.com" {
		t.Fatalf("expected external URL from env, got %q", cfg.Server.ExternalURL)
	}
}

func TestLoadReadsSecuritySecretsFromEnv(t *testing.T) {
	t.Setenv("BACKUPX_SECURITY_JWT_SECRET", "test-jwt-secret")
	t.Setenv("BACKUPX_SECURITY_ENCRYPTION_KEY", "test-encryption-key")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Security.JWTSecret != "test-jwt-secret" {
		t.Fatalf("expected JWT secret from env, got %q", cfg.Security.JWTSecret)
	}
	if cfg.Security.EncryptionKey != "test-encryption-key" {
		t.Fatalf("expected encryption key from env, got %q", cfg.Security.EncryptionKey)
	}
}

func TestLoadReadsTrustedProxiesFromEnv(t *testing.T) {
	t.Setenv("BACKUPX_SERVER_TRUSTED_PROXIES", "127.0.0.1,172.18.0.0/16")
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Server.TrustedProxies) != 2 || cfg.Server.TrustedProxies[1] != "172.18.0.0/16" {
		t.Fatalf("trusted proxies = %#v", cfg.Server.TrustedProxies)
	}
}

func TestLoadAllowsTrustedProxiesToBeDisabled(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  trusted_proxies: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Server.TrustedProxies) != 0 {
		t.Fatalf("trusted proxies should be disabled, got %#v", cfg.Server.TrustedProxies)
	}
}
