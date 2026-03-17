package config

import "testing"

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
