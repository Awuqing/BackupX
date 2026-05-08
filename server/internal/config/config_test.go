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
