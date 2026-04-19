package installscript

import (
	"strings"
	"testing"

	"backupx/server/internal/model"
)

var testCtx = Context{
	MasterURL:     "https://master.example.com",
	AgentToken:    "test-token-hex",
	AgentVersion:  "v1.7.0",
	Mode:          model.InstallModeSystemd,
	Arch:          model.InstallArchAuto,
	DownloadBase:  "https://github.com/Awuqing/BackupX/releases/download",
	InstallPrefix: "/opt/backupx-agent",
	NodeID:        42,
}

func TestRenderScriptSystemd(t *testing.T) {
	got, err := RenderScript(testCtx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	mustContain := []string{
		"BACKUPX_AGENT_MASTER=${MASTER_URL}",
		`Environment="BACKUPX_AGENT_TOKEN=${AGENT_TOKEN}"`,
		"systemctl daemon-reload",
		"systemctl enable --now backupx-agent",
		"X-Agent-Token: ${AGENT_TOKEN}",
		"MASTER_URL=\"https://master.example.com\"",
		"AGENT_TOKEN=\"test-token-hex\"",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("systemd script missing %q", s)
		}
	}
	mustNotContain := []string{"docker run", `exec "${INSTALL_PREFIX}/backupx" agent --temp-dir`}
	for _, s := range mustNotContain {
		if strings.Contains(got, s) {
			t.Errorf("systemd script unexpectedly contains %q", s)
		}
	}
}

func TestRenderScriptForeground(t *testing.T) {
	ctx := testCtx
	ctx.Mode = model.InstallModeForeground
	got, err := RenderScript(ctx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	if !strings.Contains(got, `exec "${INSTALL_PREFIX}/backupx" agent`) {
		t.Errorf("foreground script missing exec line:\n%s", got)
	}
	if strings.Contains(got, "systemctl daemon-reload") {
		t.Errorf("foreground script should not reference systemctl:\n%s", got)
	}
	if strings.Contains(got, "docker run") {
		t.Errorf("foreground script should not reference docker:\n%s", got)
	}
}

func TestRenderScriptDocker(t *testing.T) {
	ctx := testCtx
	ctx.Mode = model.InstallModeDocker
	got, err := RenderScript(ctx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	if !strings.Contains(got, "docker run") {
		t.Errorf("docker script missing `docker run`:\n%s", got)
	}
	if !strings.Contains(got, "awuqing/backupx:${AGENT_VERSION}") {
		t.Errorf("docker script missing image tag reference:\n%s", got)
	}
	if strings.Contains(got, "systemctl daemon-reload") {
		t.Errorf("docker script should not reference systemctl:\n%s", got)
	}
}

func TestRenderComposeYaml(t *testing.T) {
	ctx := testCtx
	ctx.Mode = model.InstallModeDocker
	got, err := RenderComposeYaml(ctx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	if !strings.Contains(got, "image: awuqing/backupx:v1.7.0") {
		t.Errorf("compose missing image:\n%s", got)
	}
	if !strings.Contains(got, `BACKUPX_AGENT_TOKEN: "test-token-hex"`) {
		t.Errorf("compose missing token env:\n%s", got)
	}
}

func TestDownloadBaseMapping(t *testing.T) {
	cases := map[string]string{
		model.InstallSourceGitHub:  "https://github.com/Awuqing/BackupX/releases/download",
		model.InstallSourceGhproxy: "https://ghproxy.com/https://github.com/Awuqing/BackupX/releases/download",
	}
	for src, want := range cases {
		got := DownloadBaseFor(src)
		if got != want {
			t.Errorf("src=%s want=%s got=%s", src, want, got)
		}
	}
}

func TestRenderScriptDefaultsApplied(t *testing.T) {
	ctx := testCtx
	ctx.InstallPrefix = ""   // 应被默认为 /opt/backupx-agent
	ctx.DownloadBase = ""    // 应被默认为 github
	got, err := RenderScript(ctx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	if !strings.Contains(got, "INSTALL_PREFIX=\"/opt/backupx-agent\"") {
		t.Errorf("default InstallPrefix not applied:\n%s", got)
	}
	if !strings.Contains(got, "DOWNLOAD_BASE=\"https://github.com/Awuqing/BackupX/releases/download\"") {
		t.Errorf("default DownloadBase not applied:\n%s", got)
	}
}
