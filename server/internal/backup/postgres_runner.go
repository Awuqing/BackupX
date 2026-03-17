//go:build ignore

package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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

func (r *PostgreSQLRunner) Run(ctx context.Context, spec TaskSpec, logger LogSink) (*Result, error) {
	if _, err := r.executor.LookPath("pg_dump"); err != nil {
		return nil, fmt.Errorf("pg_dump is required: %w", err)
	}
	databases := splitDatabaseNames(spec.DBName)
	if len(databases) == 0 {
		return nil, fmt.Errorf("postgresql database name is required")
	}
	tempDir, err := CreateTaskTempDir(spec.TaskName, spec.StartedAt)
	if err != nil {
		return nil, err
	}
	if len(databases) == 1 {
		return r.dumpSingleDatabase(ctx, spec, databases[0], tempDir, logger)
	}
	multiDumpDir := filepath.Join(tempDir, "postgres-dumps")
	if err := os.MkdirAll(multiDumpDir, 0o755); err != nil {
		return nil, fmt.Errorf("create postgres multi dump directory: %w", err)
	}
	for _, databaseName := range databases {
		if _, err := r.dumpDatabaseToFile(ctx, spec, databaseName, filepath.Join(multiDumpDir, sanitizeDumpName(databaseName)+".sql"), logger); err != nil {
			return nil, err
		}
	}
	fileName := BuildArtifactName(spec.TaskName, spec.StartedAt, "tar.gz")
	artifactPath := filepath.Join(tempDir, fileName)
	size, err := CreateTarGz(ctx, multiDumpDir, nil, artifactPath, logger)
	if err != nil {
		return nil, err
	}
	return &Result{ArtifactPath: artifactPath, FileName: fileName, Size: size, StorageKey: BuildStorageKey("postgresql", spec.StartedAt, fileName)}, nil
}

func (r *PostgreSQLRunner) Restore(ctx context.Context, spec TaskSpec, artifactPath string, logger LogSink) error {
	if _, err := r.executor.LookPath("psql"); err != nil {
		return fmt.Errorf("psql is required: %w", err)
	}
	databases := splitDatabaseNames(spec.DBName)
	if len(databases) == 0 {
		return fmt.Errorf("postgresql database name is required")
	}
	if strings.HasSuffix(strings.ToLower(artifactPath), ".tar.gz") {
		restoreDir, err := CreateTaskTempDir(spec.TaskName+"-restore", spec.StartedAt)
		if err != nil {
			return err
		}
		if err := ExtractTarGz(ctx, artifactPath, restoreDir, logger); err != nil {
			return err
		}
		for _, databaseName := range databases {
			filePath := filepath.Join(restoreDir, filepath.Base(restoreDir), sanitizeDumpName(databaseName)+".sql")
			if _, err := os.Stat(filePath); err != nil {
				fallback := filepath.Join(restoreDir, "postgres-dumps", sanitizeDumpName(databaseName)+".sql")
				filePath = fallback
			}
			if err := r.restoreDatabaseFromFile(ctx, spec, databaseName, filePath, logger); err != nil {
				return err
			}
		}
		return nil
	}
	return r.restoreDatabaseFromFile(ctx, spec, databases[0], artifactPath, logger)
}

func (r *PostgreSQLRunner) dumpSingleDatabase(ctx context.Context, spec TaskSpec, databaseName string, tempDir string, logger LogSink) (*Result, error) {
	fileName := BuildArtifactName(spec.TaskName, spec.StartedAt, "sql")
	artifactPath := filepath.Join(tempDir, fileName)
	size, err := r.dumpDatabaseToFile(ctx, spec, databaseName, artifactPath, logger)
	if err != nil {
		return nil, err
	}
	return &Result{ArtifactPath: artifactPath, FileName: fileName, Size: size, StorageKey: BuildStorageKey("postgresql", spec.StartedAt, fileName)}, nil
}

func (r *PostgreSQLRunner) dumpDatabaseToFile(ctx context.Context, spec TaskSpec, databaseName string, artifactPath string, logger LogSink) (int64, error) {
	output, err := os.Create(filepath.Clean(artifactPath))
	if err != nil {
		return 0, fmt.Errorf("create postgres dump file: %w", err)
	}
	defer output.Close()
	stderr := &bytes.Buffer{}
	args := []string{"-h", spec.DBHost, "-p", fmt.Sprintf("%d", spec.DBPort), "-U", spec.DBUser, "-d", databaseName, "--no-owner", "--no-privileges"}
	if logger != nil {
		logger.Infof("开始执行 pg_dump：%s", databaseName)
	}
	if err := r.executor.Run(ctx, "pg_dump", args, postgresEnv(spec.DBPassword), nil, output, stderr); err != nil {
		return 0, fmt.Errorf("run pg_dump: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	info, err := output.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat postgres dump file: %w", err)
	}
	return info.Size(), nil
}

func (r *PostgreSQLRunner) restoreDatabaseFromFile(ctx context.Context, spec TaskSpec, databaseName string, artifactPath string, logger LogSink) error {
	input, err := os.Open(filepath.Clean(artifactPath))
	if err != nil {
		return fmt.Errorf("open postgres restore file: %w", err)
	}
	defer input.Close()
	stderr := &bytes.Buffer{}
	args := []string{"-h", spec.DBHost, "-p", fmt.Sprintf("%d", spec.DBPort), "-U", spec.DBUser, "-d", databaseName}
	if logger != nil {
		logger.Infof("开始执行 psql 恢复：%s", databaseName)
	}
	if err := r.executor.Run(ctx, "psql", args, postgresEnv(spec.DBPassword), input, nil, stderr); err != nil {
		return fmt.Errorf("run psql restore: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func postgresEnv(password string) map[string]string {
	if strings.TrimSpace(password) == "" {
		return nil
	}
	return map[string]string{"PGPASSWORD": password}
}

func splitDatabaseNames(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func sanitizeDumpName(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	trimmed = strings.ReplaceAll(trimmed, " ", "-")
	trimmed = strings.ReplaceAll(trimmed, "/", "-")
	trimmed = strings.ReplaceAll(trimmed, "\\", "-")
	trimmed = strings.Trim(trimmed, "-._")
	if trimmed == "" {
		return "database"
	}
	return trimmed
}
