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
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是 Agent 的运行时配置。
type Config struct {
	// Master BackupX Master 的 HTTP 基础地址，例如 http://master.example.com:8340
	Master string `yaml:"master"`
	// Token 节点认证令牌（在 Master 创建节点时生成）
	Token string `yaml:"token"`
	// HeartbeatInterval 心跳间隔，默认 15s
	HeartbeatInterval string `yaml:"heartbeatInterval"`
	// PollInterval 命令轮询间隔，默认 5s
	PollInterval string `yaml:"pollInterval"`
	// TempDir 备份临时目录，默认 /tmp/backupx-agent
	TempDir string `yaml:"tempDir"`
	// InsecureSkipTLSVerify 测试环境允许跳过 TLS 证书校验
	InsecureSkipTLSVerify bool `yaml:"insecureSkipTlsVerify"`
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
//   - BACKUPX_AGENT_HEARTBEAT         心跳间隔（如 15s）
//   - BACKUPX_AGENT_POLL              命令轮询间隔（如 5s）
//   - BACKUPX_AGENT_TEMP_DIR          临时目录
//   - BACKUPX_AGENT_INSECURE_TLS      true / 1 跳过 TLS 校验
func LoadConfigFromEnv() (*Config, error) {
	cfg := &Config{
		Master:                strings.TrimSpace(os.Getenv("BACKUPX_AGENT_MASTER")),
		Token:                 strings.TrimSpace(os.Getenv("BACKUPX_AGENT_TOKEN")),
		HeartbeatInterval:     strings.TrimSpace(os.Getenv("BACKUPX_AGENT_HEARTBEAT")),
		PollInterval:          strings.TrimSpace(os.Getenv("BACKUPX_AGENT_POLL")),
		TempDir:               strings.TrimSpace(os.Getenv("BACKUPX_AGENT_TEMP_DIR")),
		InsecureSkipTLSVerify: strings.EqualFold(os.Getenv("BACKUPX_AGENT_INSECURE_TLS"), "true") || os.Getenv("BACKUPX_AGENT_INSECURE_TLS") == "1",
	}
	return applyConfigDefaults(cfg)
}

// MergeWithFlags 把命令行覆盖值合并入配置（非空覆盖）。
func (c *Config) MergeWithFlags(master, token, tempDir string) {
	if strings.TrimSpace(master) != "" {
		c.Master = master
	}
	if strings.TrimSpace(token) != "" {
		c.Token = token
	}
	if strings.TrimSpace(tempDir) != "" {
		c.TempDir = tempDir
	}
}

// Validate 校验必填字段。
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Master) == "" {
		return errors.New("master url is required (set via --master, BACKUPX_AGENT_MASTER or config file)")
	}
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("token is required (set via --token, BACKUPX_AGENT_TOKEN or config file)")
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
		cfg.TempDir = "/tmp/backupx-agent"
	}
	cfg.Master = strings.TrimRight(strings.TrimSpace(cfg.Master), "/")
	return cfg, nil
}
