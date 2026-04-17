package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"backupx/server/internal/agent"
)

// runAgent 是 `backupx agent` 子命令入口。
//
// 用法：
//
//	backupx agent --master http://master:8340 --token <token>
//	backupx agent --config /etc/backupx-agent.yaml
//
// 配置优先级：CLI 参数 > 配置文件 > 环境变量
func runAgent(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	configPath := fs.String("config", "", "path to agent config YAML (optional)")
	master := fs.String("master", "", "master URL, e.g. http://master.example.com:8340")
	token := fs.String("token", "", "agent authentication token")
	tempDir := fs.String("temp-dir", "", "local temp directory for backup artifacts")
	insecureTLS := fs.Bool("insecure-tls", false, "skip TLS verification (testing only)")

	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	cfg, err := loadAgentConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent: load config: %v\n", err)
		os.Exit(2)
	}
	cfg.MergeWithFlags(*master, *token, *tempDir)
	if *insecureTLS {
		cfg.InsecureSkipTLSVerify = true
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "agent: %v\n", err)
		os.Exit(2)
	}

	a, err := agent.New(cfg, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "agent: init: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(os.Stderr, "backupx agent %s starting (master=%s)\n", version, cfg.Master)
	if err := a.Run(ctx); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "agent: %v\n", err)
		os.Exit(1)
	}
}

// loadAgentConfig 按优先级加载配置：如果提供了 --config 就用文件，否则走环境变量。
func loadAgentConfig(configPath string) (*agent.Config, error) {
	if configPath != "" {
		return agent.LoadConfigFile(configPath)
	}
	return agent.LoadConfigFromEnv()
}
