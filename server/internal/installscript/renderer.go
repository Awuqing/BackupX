// Package installscript 负责把一次性安装令牌 + 节点配置渲染为可执行 shell 脚本或 docker-compose YAML。
//
// 模板文件通过 go:embed 嵌入二进制，避免运行时依赖外部资源。
package installscript

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/url"
	"strings"
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
	if err := validateContext(ctx); err != nil {
		return "", err
	}
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
	if err := validateContext(ctx); err != nil {
		return "", err
	}
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

// validateContext 对模板变量做安全校验，防止 YAML/shell 注入。
//   - MasterURL：必须是合法 http(s) URL，无控制字符
//   - AgentToken：仅允许 hex 字符，最长 128
//   - AgentVersion：仅允许 tag 常见字符（字母数字、点、连字符、下划线、加号）
//
// 这些字段被直接写入 shell 双引号字符串和 YAML 双引号值；不做校验会带来
// 注入风险（如 MasterURL 含 `"\nCOMMAND:` 可逃逸 YAML 结构）。
func validateContext(ctx Context) error {
	if err := validateMasterURL(ctx.MasterURL); err != nil {
		return err
	}
	if err := validateAgentToken(ctx.AgentToken); err != nil {
		return err
	}
	if err := validateAgentVersion(ctx.AgentVersion); err != nil {
		return err
	}
	return nil
}

func validateMasterURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("master URL empty")
	}
	if strings.ContainsAny(raw, " \t\r\n\"'`$\\") {
		return fmt.Errorf("master URL contains illegal characters")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid master URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("master URL scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("master URL missing host")
	}
	return nil
}

// validateAgentToken 允许占位符 <AGENT_TOKEN>（PreviewScript 使用），
// 或 32 字节 hex（64 字符）+ 小幅兼容（16-128 hex 字符）
func validateAgentToken(tok string) error {
	if tok == "<AGENT_TOKEN>" {
		return nil
	}
	if len(tok) < 8 || len(tok) > 128 {
		return fmt.Errorf("agent token length out of range")
	}
	for _, c := range tok {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return fmt.Errorf("agent token must be hex")
		}
	}
	return nil
}

func validateAgentVersion(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return fmt.Errorf("agent version empty")
	}
	if len(v) > 64 {
		return fmt.Errorf("agent version too long")
	}
	for _, c := range v {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c == '.' || c == '-' || c == '_' || c == '+':
		default:
			return fmt.Errorf("agent version contains illegal char %q", c)
		}
	}
	return nil
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
