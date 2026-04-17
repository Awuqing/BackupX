package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"backupx/server/internal/app"
	"backupx/server/internal/config"
	"backupx/server/internal/security"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var version = "dev"

func main() {
	// 子命令分发：reset-password
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		runResetPassword(os.Args[2:])
		return
	}
	// 子命令分发：backint（SAP HANA Backint Agent 模式）
	if len(os.Args) > 1 && os.Args[1] == "backint" {
		runBackint(os.Args[2:])
		return
	}
	// 子命令分发：agent（远程节点 Agent 模式）
	if len(os.Args) > 1 && os.Args[1] == "agent" {
		runAgent(os.Args[2:])
		return
	}

	var configPath string
	var showVersion bool

	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.BoolVar(&showVersion, "version", false, "print version")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap app: %v\n", err)
		os.Exit(1)
	}
	defer application.Close()

	if err := application.Run(ctx); err != nil {
		application.Logger().Error("application exited with error", app.ErrorField(err))
		os.Exit(1)
	}
}

// runResetPassword 通过 CLI 直接操作 SQLite 重置用户密码，无需完整 app 初始化。
// 用法：backupx reset-password --username admin --password newpass123 [--config path]
func runResetPassword(args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	username := fs.String("username", "admin", "要重置密码的用户名")
	password := fs.String("password", "", "新密码（至少 8 个字符）")
	configPath := fs.String("config", "", "配置文件路径")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *password == "" {
		fmt.Fprintln(os.Stderr, "错误：--password 参数为必填项")
		fs.Usage()
		os.Exit(1)
	}
	if len(*password) < 8 {
		fmt.Fprintln(os.Stderr, "错误：密码长度至少 8 个字符")
		os.Exit(1)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败：%v\n", err)
		os.Exit(1)
	}

	db, err := gorm.Open(sqlite.Open(cfg.Database.Path), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开数据库失败：%v\n", err)
		os.Exit(1)
	}

	var count int64
	db.Table("users").Where("username = ?", *username).Count(&count)
	if count == 0 {
		fmt.Fprintf(os.Stderr, "错误：用户 %q 不存在\n", *username)
		os.Exit(1)
	}

	hash, err := security.HashPassword(*password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "密码哈希失败：%v\n", err)
		os.Exit(1)
	}

	result := db.Table("users").Where("username = ?", *username).Update("password_hash", hash)
	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "密码更新失败：%v\n", result.Error)
		os.Exit(1)
	}

	fmt.Printf("用户 %q 密码已重置成功\n", *username)
}
