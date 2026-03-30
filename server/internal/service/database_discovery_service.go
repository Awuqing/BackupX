package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/backup"
)

type DatabaseDiscoverInput struct {
	Type     string `json:"type" binding:"required,oneof=mysql postgresql"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required,min=1"`
	User     string `json:"user" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type DatabaseDiscoverResult struct {
	Databases []string `json:"databases"`
}

type DatabaseDiscoveryService struct {
	executor backup.CommandExecutor
}

func NewDatabaseDiscoveryService(executor backup.CommandExecutor) *DatabaseDiscoveryService {
	return &DatabaseDiscoveryService{executor: executor}
}

func (s *DatabaseDiscoveryService) Discover(ctx context.Context, input DatabaseDiscoverInput) (*DatabaseDiscoverResult, error) {
	switch strings.TrimSpace(strings.ToLower(input.Type)) {
	case "mysql":
		return s.discoverMySQL(ctx, input)
	case "postgresql":
		return s.discoverPostgreSQL(ctx, input)
	default:
		return nil, apperror.BadRequest("DATABASE_DISCOVER_INVALID_TYPE", "不支持的数据库类型", nil)
	}
}

func (s *DatabaseDiscoveryService) discoverMySQL(ctx context.Context, input DatabaseDiscoverInput) (*DatabaseDiscoverResult, error) {
	mysqlPath, err := s.executor.LookPath("mysql")
	if err != nil {
		return nil, apperror.BadRequest("DATABASE_DISCOVER_MYSQL_NOT_FOUND", "系统未安装 mysql 客户端", err)
	}

	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	args := []string{
		fmt.Sprintf("--host=%s", input.Host),
		fmt.Sprintf("--port=%d", input.Port),
		fmt.Sprintf("--user=%s", input.User),
		"-e", "SHOW DATABASES",
		"--skip-column-names",
	}
	env := []string{fmt.Sprintf("MYSQL_PWD=%s", input.Password)}

	if err := s.executor.Run(timeout, mysqlPath, args, backup.CommandOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Env:    env,
	}); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, apperror.BadRequest("DATABASE_DISCOVER_MYSQL_FAILED", fmt.Sprintf("连接 MySQL 失败：%s", sanitizeMessage(errMsg)), err)
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

	return &DatabaseDiscoverResult{Databases: databases}, nil
}

func (s *DatabaseDiscoveryService) discoverPostgreSQL(ctx context.Context, input DatabaseDiscoverInput) (*DatabaseDiscoverResult, error) {
	psqlPath, err := s.executor.LookPath("psql")
	if err != nil {
		return nil, apperror.BadRequest("DATABASE_DISCOVER_PSQL_NOT_FOUND", "系统未安装 psql 客户端", err)
	}

	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	args := []string{
		"-h", input.Host,
		"-p", fmt.Sprintf("%d", input.Port),
		"-U", input.User,
		"-d", "postgres",
		"-t", "-A",
		"-c", "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname",
	}
	env := []string{fmt.Sprintf("PGPASSWORD=%s", input.Password)}

	if err := s.executor.Run(timeout, psqlPath, args, backup.CommandOptions{
		Stdout: &stdout,
		Stderr: &stderr,
		Env:    env,
	}); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = err.Error()
		}
		return nil, apperror.BadRequest("DATABASE_DISCOVER_PSQL_FAILED", fmt.Sprintf("连接 PostgreSQL 失败：%s", sanitizeMessage(errMsg)), err)
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

	return &DatabaseDiscoverResult{Databases: databases}, nil
}
