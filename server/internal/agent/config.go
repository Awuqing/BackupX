// Package agent 实现 BackupX 远程 Agent。
//
// Agent 是一个独立的 Go 进程，部署在远程服务器上，通过 HTTP 轮询的方式
// 与 Master 通信：定期上报心跳、拉取 Master 下发的命令、本地执行备份、
// 把执行结果和日志回报给 Master。
//
// 通信协议见 server/internal/http/agent_handler.go。
package agent

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是 Agent 的运行时配置。
type Config struct {
	// Master BackupX Master 的 HTTP 基础地址，例如 http://master.example.com:8340
	Master string `yaml:"master"`
	// Token 节点认证令牌（在 Master 创建节点时生成）
	Token string `yaml:"token"`
	// TokenFile 从文件读取节点认证令牌；适合 systemd 凭据和容器 secret。
	// Token 与 TokenFile 同时设置时优先使用 Token。
	TokenFile string `yaml:"tokenFile"`
	// HeartbeatInterval 心跳间隔，默认 15s
	HeartbeatInterval string `yaml:"heartbeatInterval"`
	// PollInterval 命令轮询间隔，默认 5s
	PollInterval string `yaml:"pollInterval"`
	// TempDir 备份临时目录，默认 /var/lib/backupx-agent/tmp
	TempDir string `yaml:"tempDir"`
	// ProxyURL Agent 访问 Master 使用的显式代理。留空时遵循
	// HTTP_PROXY、HTTPS_PROXY 与 NO_PROXY；支持 http(s) 和 socks5(h)。
	ProxyURL string `yaml:"proxyUrl"`
	// CACertFile 私有 CA 的 PEM 文件路径，用于安全连接内网 HTTPS Master。
	CACertFile string `yaml:"caCertFile"`
	// InsecureSkipTLSVerify 测试环境允许跳过 TLS 证书校验
	InsecureSkipTLSVerify bool `yaml:"insecureSkipTlsVerify"`
}

// Overrides 表示命令行显式提供的 Agent 配置覆盖项。
type Overrides struct {
	Master     string
	Token      string
	TokenFile  string
	TempDir    string
	ProxyURL   string
	CACertFile string
}

// LoadConfigFile 从 YAML 文件加载 Agent 配置。
func LoadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent config: %w", err)
	}
	return applyConfigDefaults(&cfg)
}

// LoadConfigFromEnv 从环境变量加载 Agent 配置。优先级低于 --config 文件。
//
// 支持的环境变量：
//   - BACKUPX_AGENT_MASTER            Master URL
//   - BACKUPX_AGENT_TOKEN             节点认证令牌
//   - BACKUPX_AGENT_TOKEN_FILE        节点认证令牌文件
//   - BACKUPX_AGENT_HEARTBEAT         心跳间隔（如 15s）
//   - BACKUPX_AGENT_POLL              命令轮询间隔（如 5s）
//   - BACKUPX_AGENT_TEMP_DIR          临时目录
//   - BACKUPX_AGENT_PROXY_URL         显式 HTTP(S)/SOCKS5 代理
//   - BACKUPX_AGENT_CA_CERT_FILE      私有 CA PEM 文件
//   - BACKUPX_AGENT_INSECURE_TLS      true / 1 跳过 TLS 校验
func LoadConfigFromEnv() (*Config, error) {
	cfg := &Config{
		Master:                strings.TrimSpace(os.Getenv("BACKUPX_AGENT_MASTER")),
		Token:                 strings.TrimSpace(os.Getenv("BACKUPX_AGENT_TOKEN")),
		TokenFile:             strings.TrimSpace(os.Getenv("BACKUPX_AGENT_TOKEN_FILE")),
		HeartbeatInterval:     strings.TrimSpace(os.Getenv("BACKUPX_AGENT_HEARTBEAT")),
		PollInterval:          strings.TrimSpace(os.Getenv("BACKUPX_AGENT_POLL")),
		TempDir:               strings.TrimSpace(os.Getenv("BACKUPX_AGENT_TEMP_DIR")),
		ProxyURL:              strings.TrimSpace(os.Getenv("BACKUPX_AGENT_PROXY_URL")),
		CACertFile:            strings.TrimSpace(os.Getenv("BACKUPX_AGENT_CA_CERT_FILE")),
		InsecureSkipTLSVerify: strings.EqualFold(os.Getenv("BACKUPX_AGENT_INSECURE_TLS"), "true") || os.Getenv("BACKUPX_AGENT_INSECURE_TLS") == "1",
	}
	return applyConfigDefaults(cfg)
}

// ApplyOverrides 把命令行覆盖值合并入配置（非空覆盖）。
func (c *Config) ApplyOverrides(overrides Overrides) {
	if strings.TrimSpace(overrides.Master) != "" {
		c.Master = strings.TrimSpace(overrides.Master)
	}
	tokenProvided := strings.TrimSpace(overrides.Token) != ""
	if strings.TrimSpace(overrides.TokenFile) != "" {
		c.TokenFile = strings.TrimSpace(overrides.TokenFile)
		if !tokenProvided {
			// An explicit --token-file must override a token inherited from YAML.
			c.Token = ""
		}
	}
	if tokenProvided {
		c.Token = strings.TrimSpace(overrides.Token)
	}
	if strings.TrimSpace(overrides.TempDir) != "" {
		c.TempDir = strings.TrimSpace(overrides.TempDir)
	}
	if strings.TrimSpace(overrides.ProxyURL) != "" {
		c.ProxyURL = strings.TrimSpace(overrides.ProxyURL)
	}
	if strings.TrimSpace(overrides.CACertFile) != "" {
		c.CACertFile = strings.TrimSpace(overrides.CACertFile)
	}
}

// ResolveToken 在所有配置源合并完成后读取 token 文件。
func (c *Config) ResolveToken() error {
	if strings.TrimSpace(c.Token) != "" || strings.TrimSpace(c.TokenFile) == "" {
		c.Token = strings.TrimSpace(c.Token)
		return nil
	}
	data, err := os.ReadFile(strings.TrimSpace(c.TokenFile))
	if err != nil {
		return fmt.Errorf("read agent token file: %w", err)
	}
	c.Token = strings.TrimSpace(string(data))
	if c.Token == "" {
		return errors.New("agent token file is empty")
	}
	return nil
}

// Validate 校验必填字段。
func (c *Config) Validate() error {
	masterURL, err := url.Parse(strings.TrimSpace(c.Master))
	if strings.TrimSpace(c.Master) == "" {
		return errors.New("master url is required (set via --master, BACKUPX_AGENT_MASTER or config file)")
	}
	if err != nil || (masterURL.Scheme != "http" && masterURL.Scheme != "https") || masterURL.Host == "" || masterURL.User != nil || masterURL.RawQuery != "" || masterURL.Fragment != "" {
		return errors.New("master url must be an absolute http(s) URL without credentials, query or fragment")
	}
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("token is required (set via --token, --token-file, environment or config file)")
	}
	if c.ProxyURL != "" {
		proxyURL, proxyErr := url.Parse(c.ProxyURL)
		if proxyErr != nil || proxyURL.Host == "" {
			return errors.New("proxy url must be an absolute URL")
		}
		switch proxyURL.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return errors.New("proxy url scheme must be http, https, socks5 or socks5h")
		}
		if proxyURL.RawQuery != "" || proxyURL.Fragment != "" || (proxyURL.Path != "" && proxyURL.Path != "/") {
			return errors.New("proxy url must not contain a path, query or fragment")
		}
	}
	if c.CACertFile != "" && c.InsecureSkipTLSVerify {
		return errors.New("ca cert file and insecure TLS cannot be enabled together")
	}
	for name, value := range map[string]string{
		"heartbeat interval": c.HeartbeatInterval,
		"poll interval":      c.PollInterval,
	} {
		duration, durationErr := time.ParseDuration(value)
		if durationErr != nil || duration <= 0 {
			return fmt.Errorf("%s must be a positive duration", name)
		}
	}
	return nil
}

func applyConfigDefaults(cfg *Config) (*Config, error) {
	if cfg.HeartbeatInterval == "" {
		cfg.HeartbeatInterval = "15s"
	}
	if cfg.PollInterval == "" {
		cfg.PollInterval = "5s"
	}
	if cfg.TempDir == "" {
		cfg.TempDir = "/var/lib/backupx-agent/tmp"
	}
	cfg.Master = strings.TrimRight(strings.TrimSpace(cfg.Master), "/")
	cfg.Token = strings.TrimSpace(cfg.Token)
	cfg.TokenFile = strings.TrimSpace(cfg.TokenFile)
	cfg.ProxyURL = strings.TrimSpace(cfg.ProxyURL)
	cfg.CACertFile = strings.TrimSpace(cfg.CACertFile)
	return cfg, nil
}
