package installscript

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDeployInstallScriptSyntax(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "..", "deploy", "install.sh")
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("POSIX sh is not available on this platform")
	}
	cmd := exec.Command(sh, "-n", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh syntax invalid: %v\n%s", err, output)
	}
}

func TestDeployInstallScriptSupportsReleasePackageLayout(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "..", "deploy", "install.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		`SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)`,
		`if [ -f "$SCRIPT_DIR/backupx" ] && [ -d "$SCRIPT_DIR/web" ]; then`,
		`BIN_SOURCE="${BIN_SOURCE:-$SCRIPT_DIR/backupx}"`,
		`WEB_SOURCE="${WEB_SOURCE:-$SCRIPT_DIR/web}"`,
		`CONFIG_TEMPLATE="${CONFIG_TEMPLATE:-$SCRIPT_DIR/config.example.yaml}"`,
		`SERVICE_SOURCE_DEFAULT="$SCRIPT_DIR/backupx.service"`,
		`发布包安装请确认当前目录包含 ./backupx、./web 和 ./install.sh。`,
		`cat > "/etc/systemd/system/$SERVICE_NAME.service" <<UNIT`,
		`if [ "$INSTALL_NGINX" = "1" ]; then`,
		`[ "$PREFIX" = "/opt/backupx" ] && [ "$ETC_DIR" = "/etc/backupx" ]`,
		`validate_install_path PREFIX "$PREFIX"`,
		`拒绝通过符号链接写入受管目录`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install.sh missing %q", want)
		}
	}
}

func TestDeployInstallScriptSupportsSourceBuildAndVerifiesFirstSetup(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "..", "deploy", "install.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, want := range []string{
		`SOURCE_BIN_DEFAULT="$PROJECT_ROOT/server/bin/backupx"`,
		`For a source install, run 'make build' in the repository root first.`,
		`HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8340/api/auth/setup/status}"`,
		`systemctl is-active --quiet "$SERVICE_NAME"`,
		`System setup`,
		`chown -R root:root "$PREFIX/bin" "$PREFIX/web"`,
		`find "$PREFIX/web" -type f -exec chmod 0644`,
		`chown root:"$APP_GROUP" "$ETC_DIR/config.yaml"`,
		`systemctl restart "$SERVICE_NAME"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install.sh missing %q", want)
		}
	}
}

func TestDockerDeploymentUsesSingleUnprivilegedProcess(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	dockerfileData, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	dockerfile := string(dockerfileData)
	for _, want := range []string{"su-exec", "BACKUPX_SERVER_WEB_ROOT=/app/web", "HEALTHCHECK"} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("Dockerfile missing %q", want)
		}
	}
	for _, forbidden := range []string{"docker-cli", "COPY deploy/docker/nginx.conf"} {
		if strings.Contains(dockerfile, forbidden) {
			t.Fatalf("Dockerfile still contains %q", forbidden)
		}
	}

	composeData, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var compose map[string]any
	if err := yaml.Unmarshal(composeData, &compose); err != nil {
		t.Fatalf("docker-compose.yml is not valid YAML: %v", err)
	}
	composeText := string(composeData)
	for _, want := range []string{"no-new-privileges:true", "cap_drop:", "cap_add:", "DAC_OVERRIDE", "SETGID", "SETUID", "/ready"} {
		if !strings.Contains(composeText, want) {
			t.Fatalf("docker-compose.yml missing %q", want)
		}
	}
	if strings.Contains(composeText, "docker.sock") {
		t.Fatal("docker-compose.yml must not expose the Docker socket")
	}
}

func TestReleaseWorkflowPublishesChecksums(t *testing.T) {
	workflowPath := filepath.Join("..", "..", "..", ".github", "workflows", "release.yml")
	workflowData, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	var workflow map[string]any
	if err := yaml.Unmarshal(workflowData, &workflow); err != nil {
		t.Fatalf("release workflow is not valid YAML: %v", err)
	}
	workflowText := string(workflowData)
	for _, want := range []string{
		`cp deploy/backupx.service "${ARCHIVE_NAME}/"`,
		`sha256sum "${ARCHIVE_NAME}.tar.gz"`,
		`backupx-${{ matrix.goos }}-${{ matrix.goarch }}.tar.gz.sha256`,
	} {
		if !strings.Contains(workflowText, want) {
			t.Fatalf("release workflow missing %q", want)
		}
	}
}
