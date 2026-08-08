package installscript

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"backupx/server/internal/model"
	"gopkg.in/yaml.v3"
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
		`master: "${MASTER_URL}"`,
		`tokenFile: "${TOKEN_FILE}"`,
		`ExecStart=${INSTALL_PREFIX}/backupx agent --config ${CONFIG_FILE}`,
		"/var/lib/backupx-agent/tmp",
		"systemctl daemon-reload",
		"systemctl enable backupx-agent",
		"systemctl restart backupx-agent",
		"systemctl status backupx-agent",
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
	if strings.Contains(got, `Environment="BACKUPX_AGENT_TOKEN=`) {
		t.Errorf("systemd unit must not expose the agent token in its environment:\n%s", got)
	}
}

func TestRenderedInstallScriptSyntax(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("POSIX sh is not available on this platform")
	}
	for _, mode := range []string{model.InstallModeSystemd, model.InstallModeDocker, model.InstallModeForeground} {
		ctx := testCtx
		ctx.Mode = mode
		script, renderErr := RenderScript(ctx)
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		cmd := exec.Command(sh, "-n")
		cmd.Stdin = strings.NewReader(script)
		if output, syntaxErr := cmd.CombinedOutput(); syntaxErr != nil {
			t.Fatalf("%s installer syntax invalid: %v\n%s", mode, syntaxErr, output)
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
	if !strings.Contains(got, "/var/lib/backupx-agent/tmp") {
		t.Errorf("foreground script missing dedicated temp dir:\n%s", got)
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
	if !strings.Contains(got, "/var/lib/backupx-agent:/var/lib/backupx-agent") {
		t.Errorf("docker script missing agent data volume:\n%s", got)
	}
	if !strings.Contains(got, "awuqing/backupx:${AGENT_VERSION}") {
		t.Errorf("docker script missing image tag reference:\n%s", got)
	}
	if !strings.Contains(got, `"awuqing/backupx:${AGENT_VERSION}" agent`) {
		t.Errorf("docker script must start image in agent mode:\n%s", got)
	}
	if !strings.Contains(got, `-v /etc/backupx-agent:/etc/backupx-agent:ro`) {
		t.Errorf("docker script missing protected config mount:\n%s", got)
	}
	if !strings.Contains(got, `agent --config /etc/backupx-agent/config.yaml`) {
		t.Errorf("docker script must load the protected config file:\n%s", got)
	}
	if !strings.Contains(got, `docker logs --tail=100 backupx-agent`) {
		t.Errorf("docker script missing diagnostic log command:\n%s", got)
	}
	if !strings.Contains(got, `grep -q '"status":"online"'`) {
		t.Errorf("docker script missing online probe:\n%s", got)
	}
	if strings.Contains(got, "systemctl daemon-reload") {
		t.Errorf("docker script should not reference systemctl:\n%s", got)
	}
	if strings.Contains(got, `-e "BACKUPX_AGENT_TOKEN=`) {
		t.Errorf("docker inspect must not expose the agent token:\n%s", got)
	}
}

func TestDockerEntrypointForwardsAgentSubcommand(t *testing.T) {
	entrypointPath := filepath.Join("..", "..", "..", "deploy", "docker", "entrypoint.sh")
	got, err := os.ReadFile(entrypointPath)
	if err != nil {
		t.Fatalf("read docker entrypoint: %v", err)
	}
	script := string(got)
	if !strings.Contains(script, `exec /app/bin/backupx "$@"`) {
		t.Fatalf("entrypoint must exec backupx with forwarded args:\n%s", script)
	}
	if !strings.Contains(script, `exec su-exec backupx:backupx /app/bin/backupx "$@"`) {
		t.Fatalf("master entrypoint must drop privileges after data migration:\n%s", script)
	}
	if !strings.Contains(script, `export HOME=/app`) {
		t.Fatalf("master entrypoint must set the service user's home directory:\n%s", script)
	}
	if strings.Contains(script, "nginx") || strings.Contains(script, "wait -n") {
		t.Fatalf("entrypoint should run a single foreground process:\n%s", script)
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
	if !strings.Contains(got, `command: ["agent"]`) {
		t.Errorf("compose must start image in agent mode:\n%s", got)
	}
	if !strings.Contains(got, `BACKUPX_AGENT_TOKEN: "deadbeefcafebabe0123456789abcdef0123456789abcdef0123456789abcdef"`) {
		t.Errorf("compose missing token env:\n%s", got)
	}
	if !strings.Contains(got, `BACKUPX_AGENT_TEMP_DIR: "/var/lib/backupx-agent/tmp"`) {
		t.Errorf("compose missing temp dir env:\n%s", got)
	}
	if !strings.Contains(got, `user: "0:0"`) || !strings.Contains(got, "no-new-privileges:true") {
		t.Errorf("compose missing root execution declaration or security option:\n%s", got)
	}
	if !strings.Contains(got, "/var/lib/backupx-agent:/var/lib/backupx-agent") {
		t.Errorf("compose missing agent data volume:\n%s", got)
	}
	var document map[string]any
	if err := yaml.Unmarshal([]byte(got), &document); err != nil {
		t.Fatalf("compose is not valid YAML: %v\n%s", err, got)
	}
}

func TestRenderComposeYamlIncludesRestrictedNetworkSettings(t *testing.T) {
	ctx := testCtx
	ctx.Mode = model.InstallModeDocker
	ctx.ProxyURL = "socks5h://127.0.0.1:1080"
	ctx.CACertFile = "/etc/backupx-agent/ca.pem"
	got, err := RenderComposeYaml(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`BACKUPX_AGENT_PROXY_URL: "socks5h://127.0.0.1:1080"`,
		`BACKUPX_AGENT_CA_CERT_FILE: "/etc/backupx-agent/ca.pem"`,
		`- /etc/backupx-agent/ca.pem:/etc/backupx-agent/ca.pem:ro`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("compose missing %q:\n%s", want, got)
		}
	}
	var document map[string]any
	if err := yaml.Unmarshal([]byte(got), &document); err != nil {
		t.Fatalf("compose is not valid YAML: %v\n%s", err, got)
	}
}

func TestRenderScriptRejectsInjectedMasterURL(t *testing.T) {
	bad := []string{
		"https://example.com\" other: inject", // 含引号和空格
		"javascript:alert(1)",                 // scheme 非法
		"https://example.com\n- privileged",   // 含换行，YAML 注入经典 payload
		"",                                    // 空
	}
	for _, u := range bad {
		ctx := testCtx
		ctx.MasterURL = u
		if _, err := RenderScript(ctx); err == nil {
			t.Errorf("RenderScript should reject MasterURL %q", u)
		}
	}
}

func TestRenderScriptIncludesRestrictedNetworkSettings(t *testing.T) {
	ctx := testCtx
	ctx.MasterURL = "http://127.0.0.1:18340"
	ctx.ProxyURL = "socks5h://127.0.0.1:1080"
	ctx.CACertFile = "/etc/pki/internal-ca.pem"
	got, err := RenderScript(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`PROXY_URL="socks5h://127.0.0.1:1080"`,
		`CA_CERT_FILE="/etc/pki/internal-ca.pem"`,
		`proxyUrl: "${PROXY_URL}"`,
		`caCertFile: "${CA_CERT_FILE}"`,
		`curl -fsSL --retry 3 --retry-delay 2 --connect-timeout 15 --proxy "$PROXY_URL"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("restricted network script missing %q", want)
		}
	}
}

func TestRenderScriptRejectsUnsafeRestrictedNetworkSettings(t *testing.T) {
	for _, mutate := range []func(*Context){
		func(ctx *Context) { ctx.ProxyURL = "ftp://proxy.example.com" },
		func(ctx *Context) { ctx.ProxyURL = "http://user:pass@proxy.example.com" },
		func(ctx *Context) { ctx.ProxyURL = "http://proxy.example.com/connect" },
		func(ctx *Context) { ctx.CACertFile = "relative-ca.pem" },
	} {
		ctx := testCtx
		mutate(&ctx)
		if _, err := RenderScript(ctx); err == nil {
			t.Fatalf("expected restricted network settings to be rejected: %+v", ctx)
		}
	}
}

func TestRenderScriptRejectsUnsafeDeploymentSettings(t *testing.T) {
	for _, mutate := range []func(*Context){
		func(ctx *Context) { ctx.Mode = "unknown" },
		func(ctx *Context) { ctx.Arch = "386" },
		func(ctx *Context) { ctx.InstallPrefix = "/opt/backupx;touch/tmp/pwned" },
		func(ctx *Context) { ctx.DownloadBase = "https://user:pass@example.com/releases" },
	} {
		ctx := testCtx
		mutate(&ctx)
		if _, err := RenderScript(ctx); err == nil {
			t.Fatalf("expected unsafe deployment settings to be rejected: %+v", ctx)
		}
	}
}

func TestRenderScriptVerifiesReleaseChecksumWithoutEmoji(t *testing.T) {
	got, err := RenderScript(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sha256sum -c", `download_file "${URL}.sha256"`, "[OK] 节点已上线", "[WARN]"} {
		if !strings.Contains(got, want) {
			t.Errorf("script missing %q", want)
		}
	}
	if strings.ContainsAny(got, "\u2713\u26a0") {
		t.Fatal("installer output must not use emoji symbols")
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
	ctx.InstallPrefix = "" // 应被默认为 /opt/backupx-agent
	ctx.DownloadBase = ""  // 应被默认为 github
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
