package backup

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"
)

// DiscoverRequest 数据库发现请求参数。
// Type 取 "mysql" 或 "postgresql"。
type DiscoverRequest struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
}

// DiscoverDatabases 通过本机 mysql/psql 客户端连接目标数据库并列出非系统库。
// 5 秒命令超时。调用方负责传入 CommandExecutor（Master 用 OSCommandExecutor，
// Agent 同理）。此函数不依赖 service / apperror，便于在 agent 包复用。
func DiscoverDatabases(ctx context.Context, executor CommandExecutor, req DiscoverRequest) ([]string, error) {
	switch strings.TrimSpace(strings.ToLower(req.Type)) {
	case "mysql":
		return discoverMySQLDatabases(ctx, executor, req)
	case "postgresql":
		return discoverPostgreSQLDatabases(ctx, executor, req)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", req.Type)
	}
}

func discoverMySQLDatabases(ctx context.Context, executor CommandExecutor, req DiscoverRequest) ([]string, error) {
	mysqlPath, err := executor.LookPath("mysql")
	if err != nil {
		return nil, fmt.Errorf("系统未安装 mysql 客户端")
	}
	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	args := []string{
		fmt.Sprintf("--host=%s", req.Host),
		fmt.Sprintf("--port=%d", req.Port),
		fmt.Sprintf("--user=%s", req.User),
		"-e", "SHOW DATABASES",
		"--skip-column-names",
	}
	env := []string{fmt.Sprintf("MYSQL_PWD=%s", req.Password)}
	if err := executor.Run(timeout, mysqlPath, args, CommandOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Env:    env,
	}); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("连接 MySQL 失败：%s", errMsg)
	}
	systemDBs := map[string]bool{
		"information_schema": true,
		"performance_schema": true,
		"mysql":              true,
		"sys":                true,
	}
	var databases []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		db := strings.TrimSpace(line)
		if db == "" || systemDBs[db] {
			continue
		}
		databases = append(databases, db)
	}
	return databases, nil
}

func discoverPostgreSQLDatabases(ctx context.Context, executor CommandExecutor, req DiscoverRequest) ([]string, error) {
	psqlPath, err := executor.LookPath("psql")
	if err != nil {
		return nil, fmt.Errorf("系统未安装 psql 客户端")
	}
	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	args := []string{
		"-h", req.Host,
		"-p", fmt.Sprintf("%d", req.Port),
		"-U", req.User,
		"-d", "postgres",
		"-t", "-A",
		"-c", "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname",
	}
	env := []string{fmt.Sprintf("PGPASSWORD=%s", req.Password)}
	if err := executor.Run(timeout, psqlPath, args, CommandOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Env:    env,
	}); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, fmt.Errorf("连接 PostgreSQL 失败：%s", errMsg)
	}
	skipDBs := map[string]bool{
		"postgres": true,
	}
	var databases []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		db := strings.TrimSpace(line)
		if db == "" || skipDBs[db] || strings.HasPrefix(db, "template") {
			continue
		}
		databases = append(databases, db)
	}
	return databases, nil
}
