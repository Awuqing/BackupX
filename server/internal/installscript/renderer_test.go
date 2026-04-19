package installscript

import (
	"strings"
	"testing"

	"backupx/server/internal/model"
)

// 使用合法 hex token（32 字节 = 64 字符）以通过 validateAgentToken 校验
var testCtx = Context{
	MasterURL:     "https://master.example.com",
	AgentToken:    "deadbeefcafebabe0123456789abcdef0123456789abcdef0123456789abcdef",
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
		"AGENT_TOKEN=\"deadbeefcafebabe0123456789abcdef0123456789abcdef0123456789abcdef\"",
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
	if !strings.Contains(got, `BACKUPX_AGENT_TOKEN: "deadbeefcafebabe0123456789abcdef0123456789abcdef0123456789abcdef"`) {
		t.Errorf("compose missing token env:\n%s", got)
	}
}

func TestRenderScriptRejectsInjectedMasterURL(t *testing.T) {
	bad := []string{
		"https://example.com\" other: inject", // 含引号和空格
		"javascript:alert(1)",                  // scheme 非法
		"https://example.com\n- privileged",    // 含换行，YAML 注入经典 payload
		"",                                     // 空
	}
	for _, u := range bad {
		ctx := testCtx
		ctx.MasterURL = u
		if _, err := RenderScript(ctx); err == nil {
			t.Errorf("RenderScript should reject MasterURL %q", u)
		}
	}
}

func TestRenderComposeYamlRejectsInjectedMasterURL(t *testing.T) {
	ctx := testCtx
	ctx.Mode = model.InstallModeDocker
	ctx.MasterURL = "https://example.com\n- privileged: true"
	if _, err := RenderComposeYaml(ctx); err == nil {
		t.Errorf("RenderComposeYaml should reject injected MasterURL")
	}
}

func TestRenderScriptRejectsBadToken(t *testing.T) {
	ctx := testCtx
	ctx.AgentToken = "not-hex-token" // 非 hex
	if _, err := RenderScript(ctx); err == nil {
		t.Errorf("should reject non-hex agent token")
	}
}

func TestRenderScriptAcceptsPlaceholderToken(t *testing.T) {
	ctx := testCtx
	ctx.AgentToken = "<AGENT_TOKEN>" // Preview 占位符
	if _, err := RenderScript(ctx); err != nil {
		t.Errorf("should accept placeholder token: %v", err)
	}
}

func TestRenderScriptRejectsBadVersion(t *testing.T) {
	ctx := testCtx
	ctx.AgentVersion = "v1.7 && rm -rf /" // 含非法字符
	if _, err := RenderScript(ctx); err == nil {
		t.Errorf("should reject version with shell metacharacters")
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
