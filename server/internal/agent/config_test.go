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
heartbeatInterval: 20s
pollInterval: 3s
tempDir: /var/backupx-agent
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
	if cfg.TempDir != "/tmp/backupx-agent" {
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
	}
	for _, c := range cases {
		err := c.cfg.Validate()
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v wantErr=%v", c.name, err, c.wantErr)
		}
	}
}

func TestMergeWithFlags(t *testing.T) {
	cfg := &Config{Master: "http://old", Token: "old"}
	cfg.MergeWithFlags("http://new", "", "/tmp/x")
	if cfg.Master != "http://new" {
		t.Errorf("master not overridden: %q", cfg.Master)
	}
	if cfg.Token != "old" {
		t.Errorf("empty flag should not override: %q", cfg.Token)
	}
	if cfg.TempDir != "/tmp/x" {
		t.Errorf("tempDir: %q", cfg.TempDir)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("BACKUPX_AGENT_MASTER", "http://env-master")
	t.Setenv("BACKUPX_AGENT_TOKEN", "env-token")
	t.Setenv("BACKUPX_AGENT_INSECURE_TLS", "true")
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Master != "http://env-master" || cfg.Token != "env-token" || !cfg.InsecureSkipTLSVerify {
		t.Errorf("env not picked up: %+v", cfg)
	}
}
