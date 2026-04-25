package installscript

import (
	"strings"
	"testing"

	"backupx/server/internal/model"
)

// TestRenderScriptIncludesMagicMarker 渲染脚本必须包含 Issue #46 引入的魔数注释，
// 方便用户通过 `head -3 脚本` 自查是否被中间层改写。
func TestRenderScriptIncludesMagicMarker(t *testing.T) {
	for _, mode := range []string{model.InstallModeSystemd, model.InstallModeDocker, model.InstallModeForeground} {
		ctx := testCtx
		ctx.Mode = mode
		got, err := RenderScript(ctx)
		if err != nil {
			t.Fatalf("render err (%s): %v", mode, err)
		}
		if !strings.Contains(got, "BACKUPX_AGENT_INSTALL_V1") {
			t.Errorf("mode=%s: script missing magic marker:\n%s", mode, got)
		}
	}
}

// TestRenderScriptBashBootstrap 脚本顶部必须有 bash 自举段，文件执行时跳到 bash。
func TestRenderScriptBashBootstrap(t *testing.T) {
	got, err := RenderScript(testCtx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	if !strings.Contains(got, `[ -z "${BASH_VERSION:-}" ]`) {
		t.Errorf("script missing bash bootstrap guard:\n%s", got)
	}
	if !strings.Contains(got, `exec bash "$0" "$@"`) {
		t.Errorf("script missing exec bash fallback:\n%s", got)
	}
}

func TestRenderScriptCreatesBackupXUserAndGroup(t *testing.T) {
	got, err := RenderScript(testCtx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	for _, want := range []string{
		"getent group backupx",
		"groupadd --system backupx",
		"useradd --system --gid backupx",
		"Group=backupx",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("script missing %q:\n%s", want, got)
		}
	}
}
