package installscript

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployInstallScriptSyntax(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "..", "deploy", "install.sh")
	cmd := exec.Command("sh", "-n", scriptPath)
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
		`发布包安装请确认当前目录包含 ./backupx、./web 和 ./install.sh。`,
		`cat > "/etc/systemd/system/$SERVICE_NAME.service" <<UNIT`,
		`if [ -d "/etc/nginx/conf.d" ] && [ -f "$NGINX_SOURCE" ]; then`,
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
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install.sh missing %q", want)
		}
	}
}
