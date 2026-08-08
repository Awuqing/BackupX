package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	content := `master: http://master.example.com:8340/
token: abc123
tokenFile: /run/secrets/backupx_agent_token
heartbeatInterval: 20s
pollInterval: 3s
tempDir: /var/backupx-agent
proxyUrl: socks5h://127.0.0.1:1080
caCertFile: /etc/backupx-agent/ca.pem
insecureSkipTlsVerify: true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Master != "http://master.example.com:8340" {
		t.Errorf("trailing slash should be trimmed: %q", cfg.Master)
	}
	if cfg.Token != "abc123" {
		t.Errorf("token: %q", cfg.Token)
	}
	if cfg.HeartbeatInterval != "20s" || cfg.PollInterval != "3s" {
		t.Errorf("intervals: %+v", cfg)
	}
	if !cfg.InsecureSkipTLSVerify {
		t.Errorf("insecure should be true")
	}
	if cfg.ProxyURL != "socks5h://127.0.0.1:1080" || cfg.CACertFile != "/etc/backupx-agent/ca.pem" {
		t.Errorf("connection options not loaded: %+v", cfg)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yaml")
	if err := os.WriteFile(path, []byte("master: http://m\ntoken: t\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HeartbeatInterval != "15s" || cfg.PollInterval != "5s" {
		t.Errorf("default intervals not applied: %+v", cfg)
	}
	if cfg.TempDir != "/var/lib/backupx-agent/tmp" {
		t.Errorf("default tempdir: %q", cfg.TempDir)
	}
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{Master: "http://m", Token: "t"}, false},
		{"missing master", Config{Token: "t"}, true},
		{"missing token", Config{Master: "http://m"}, true},
		{"invalid master scheme", Config{Master: "ssh://m", Token: "t"}, true},
		{"master credentials rejected", Config{Master: "https://user:pass@m", Token: "t"}, true},
		{"valid socks proxy", Config{Master: "https://m", Token: "t", ProxyURL: "socks5h://127.0.0.1:1080"}, false},
		{"invalid proxy", Config{Master: "https://m", Token: "t", ProxyURL: "ftp://proxy"}, true},
		{"proxy path rejected", Config{Master: "https://m", Token: "t", ProxyURL: "http://proxy/connect"}, true},
		{"invalid heartbeat", Config{Master: "https://m", Token: "t", HeartbeatInterval: "never"}, true},
	}
	for _, c := range cases {
		_, _ = applyConfigDefaults(&c.cfg)
		err := c.cfg.Validate()
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.wantErr)
		}
	}
}

func TestMergeWithFlags(t *testing.T) {
	cfg := &Config{Master: "http://old", Token: "old"}
	cfg.ApplyOverrides(Overrides{Master: "http://new", TempDir: "/tmp/x", ProxyURL: "http://proxy:3128"})
	if cfg.Master != "http://new" {
		t.Errorf("master not overridden: %q", cfg.Master)
	}
	if cfg.Token != "old" {
		t.Errorf("empty flag should not override: %q", cfg.Token)
	}
	if cfg.TempDir != "/tmp/x" {
		t.Errorf("tempDir: %q", cfg.TempDir)
	}
	if cfg.ProxyURL != "http://proxy:3128" {
		t.Errorf("proxyUrl: %q", cfg.ProxyURL)
	}
}

func TestTokenFileOverrideReplacesConfiguredToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.token")
	if err := os.WriteFile(path, []byte("file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Token: "yaml-token", TokenFile: "/old/token"}
	cfg.ApplyOverrides(Overrides{TokenFile: "  " + path + "  "})
	if err := cfg.ResolveToken(); err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "file-token" || cfg.TokenFile != path {
		t.Fatalf("token file override was not applied: %+v", cfg)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("BACKUPX_AGENT_MASTER", "http://env-master")
	t.Setenv("BACKUPX_AGENT_TOKEN", "env-token")
	t.Setenv("BACKUPX_AGENT_PROXY_URL", "http://env-proxy:8080")
	t.Setenv("BACKUPX_AGENT_INSECURE_TLS", "true")
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Master != "http://env-master" || cfg.Token != "env-token" || cfg.ProxyURL != "http://env-proxy:8080" || !cfg.InsecureSkipTLSVerify {
		t.Errorf("env not picked up: %+v", cfg)
	}
}

func TestResolveTokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.token")
	if err := os.WriteFile(path, []byte(" file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{TokenFile: path}
	if err := cfg.ResolveToken(); err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "file-token" {
		t.Fatalf("token = %q", cfg.Token)
	}
}
