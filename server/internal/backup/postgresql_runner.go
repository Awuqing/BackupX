package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type PostgreSQLRunner struct {
	executor CommandExecutor
}

func NewPostgreSQLRunner(executor CommandExecutor) *PostgreSQLRunner {
	if executor == nil {
		executor = NewOSCommandExecutor()
	}
	return &PostgreSQLRunner{executor: executor}
}

func (r *PostgreSQLRunner) Type() string {
	return "postgresql"
}

func (r *PostgreSQLRunner) Run(ctx context.Context, task TaskSpec, writer LogWriter) (*RunResult, error) {
	if _, err := r.executor.LookPath("pg_dump"); err != nil {
		return nil, fmt.Errorf("未找到 pg_dump 命令 (请确保服务器已安装 postgresql-client)")
	}
	tempDir, artifactPath, err := createTempArtifact(task.TempDir, task.Name, "sql")
	if err != nil {
		return nil, err
	}
	file, err := os.Create(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("create postgresql dump file: %w", err)
	}
	defer file.Close()
	dbNames := normalizeDatabaseNames(task.Database.Names)
	if len(dbNames) == 0 {
		return nil, fmt.Errorf("postgresql database names are required")
	}
	writer.WriteLine(fmt.Sprintf("连接到 PostgreSQL: %s:%d", task.Database.Host, task.Database.Port))
	writer.WriteLine(fmt.Sprintf("备份数据库: %s", strings.Join(dbNames, ", ")))
	stderrWriter := newLogLineWriter(writer, "pg_dump")
	for index, name := range dbNames {
		args := []string{"--clean", "--if-exists", "--create", "--format=plain", "-h", task.Database.Host, "-p", strconv.Itoa(task.Database.Port), "-U", task.Database.User, "--dbname", name}
		writer.WriteLine(fmt.Sprintf("开始导出数据库 [%d/%d]: %s", index+1, len(dbNames), name))
		if err := r.executor.Run(ctx, "pg_dump", args, CommandOptions{Stdout: file, Stderr: stderrWriter, Env: append(os.Environ(), "PGPASSWORD="+task.Database.Password)}); err != nil {
			return nil, fmt.Errorf("run pg_dump for %s: %w", name, err)
		}
		writer.WriteLine(fmt.Sprintf("数据库 %s 导出完成", name))
		if index < len(dbNames)-1 {
			if _, err := file.WriteString("\n\n"); err != nil {
				return nil, fmt.Errorf("write dump separator: %w", err)
			}
		}
	}
	info, _ := file.Stat()
	sizeStr := "未知"
	if info != nil {
		sizeStr = formatFileSize(info.Size())
	}
	writer.WriteLine(fmt.Sprintf("PostgreSQL 导出完成（文件大小: %s）", sizeStr))
	return &RunResult{ArtifactPath: artifactPath, FileName: filepath.Base(artifactPath), TempDir: tempDir}, nil
}

func (r *PostgreSQLRunner) Restore(ctx context.Context, task TaskSpec, artifactPath string, writer LogWriter) error {
	if _, err := r.executor.LookPath("psql"); err != nil {
		return fmt.Errorf("未找到 psql 命令 (请确保服务器已安装 postgresql-client)")
	}
	writer.WriteLine("开始执行 psql 恢复")
	args := []string{"-h", task.Database.Host, "-p", strconv.Itoa(task.Database.Port), "-U", task.Database.User, "-d", "postgres", "-f", artifactPath}
	if err := r.executor.Run(ctx, "psql", args, CommandOptions{Env: append(os.Environ(), "PGPASSWORD="+task.Database.Password)}); err != nil {
		return fmt.Errorf("run psql restore: %w", err)
	}
	writer.WriteLine("PostgreSQL 恢复完成")
	return nil
}
