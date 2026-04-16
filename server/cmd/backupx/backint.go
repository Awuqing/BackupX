package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"backupx/server/internal/backint"
)

// runBackint 是 `backupx backint` 子命令入口。
//
// CLI 参数遵循 SAP HANA Backint 规范：
//
//	backupx backint -f <function> -i <input-file> -o <output-file> -p <param-file>
//	                [-u <user>] [-c <config-prefix>] [-l <log-file>] [-v <version>]
//
// 除 -f / -i / -o / -p 外其余参数接受但忽略（兼容 SAP 调用约定）。
func runBackint(args []string) {
	fs := flag.NewFlagSet("backint", flag.ExitOnError)
	fnStr := fs.String("f", "", "function: backup | restore | inquire | delete")
	inputPath := fs.String("i", "", "input file path")
	outputPath := fs.String("o", "", "output file path")
	paramFile := fs.String("p", "", "parameter file path")

	// 以下参数仅为兼容 SAP 调用约定，当前未使用
	_ = fs.String("u", "", "user (ignored)")
	_ = fs.String("c", "", "config-prefix (ignored)")
	_ = fs.String("l", "", "log file override (ignored, use LOG_FILE in params)")
	_ = fs.String("v", "", "backint version (ignored)")

	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	if *fnStr == "" || *inputPath == "" || *outputPath == "" || *paramFile == "" {
		fmt.Fprintln(os.Stderr, "backint: -f, -i, -o, -p are required")
		fs.Usage()
		os.Exit(2)
	}

	fn, err := backint.ParseFunction(*fnStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backint: %v\n", err)
		os.Exit(2)
	}

	cfg, err := backint.LoadConfigFile(*paramFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backint: load config: %v\n", err)
		os.Exit(2)
	}

	// 配置日志重定向（如果指定 LOG_FILE）
	restoreLog, err := redirectStderr(cfg.LogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backint: open log: %v\n", err)
		os.Exit(2)
	}
	defer restoreLog()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	agent, err := backint.NewAgent(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backint: init agent: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = agent.Close() }()

	if err := agent.Run(ctx, fn, *inputPath, *outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "backint: run: %v\n", err)
		os.Exit(1)
	}
}

// redirectStderr 将 stderr 重定向到指定日志文件，返回恢复函数。
// 空字符串表示保持原样。
func redirectStderr(path string) (func(), error) {
	if path == "" {
		return func() {}, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	orig := os.Stderr
	os.Stderr = f
	return func() {
		os.Stderr = orig
		_ = f.Close()
	}, nil
}

