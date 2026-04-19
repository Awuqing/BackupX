// Package installscript 负责把一次性安装令牌 + 节点配置渲染为可执行 shell 脚本或 docker-compose YAML。
//
// 模板文件通过 go:embed 嵌入二进制，避免运行时依赖外部资源。
package installscript

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"backupx/server/internal/model"
)

//go:embed templates/agent-install.sh.tmpl
var installScriptTmpl string

//go:embed templates/agent-compose.yml.tmpl
var composeYamlTmpl string

// Context 是模板渲染输入。
type Context struct {
	MasterURL     string
	AgentToken    string
	AgentVersion  string
	Mode          string // systemd|docker|foreground
	Arch          string // amd64|arm64|auto
	DownloadBase  string
	InstallPrefix string
	NodeID        uint
}

// DownloadBaseFor 将下载源枚举转换为具体 URL 前缀。
func DownloadBaseFor(src string) string {
	switch src {
	case model.InstallSourceGhproxy:
		return "https://ghproxy.com/https://github.com/Awuqing/BackupX/releases/download"
	default:
		return "https://github.com/Awuqing/BackupX/releases/download"
	}
}

// RenderScript 渲染目标机安装脚本。
func RenderScript(ctx Context) (string, error) {
	ctx = withDefaults(ctx)
	tmpl, err := template.New("install").Parse(installScriptTmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// RenderComposeYaml 渲染 docker-compose.yml 片段。
func RenderComposeYaml(ctx Context) (string, error) {
	ctx = withDefaults(ctx)
	tmpl, err := template.New("compose").Parse(composeYamlTmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func withDefaults(ctx Context) Context {
	if ctx.InstallPrefix == "" {
		ctx.InstallPrefix = "/opt/backupx-agent"
	}
	if ctx.DownloadBase == "" {
		ctx.DownloadBase = DownloadBaseFor(model.InstallSourceGitHub)
	}
	return ctx
}
