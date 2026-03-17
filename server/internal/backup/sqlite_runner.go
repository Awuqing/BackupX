package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type SQLiteRunner struct{}

func NewSQLiteRunner() *SQLiteRunner {
	return &SQLiteRunner{}
}

func (r *SQLiteRunner) Type() string {
	return "sqlite"
}

func (r *SQLiteRunner) Run(_ context.Context, task TaskSpec, writer LogWriter) (*RunResult, error) {
	dbPath := filepath.Clean(strings.TrimSpace(task.Database.Path))
	if dbPath == "" {
		return nil, fmt.Errorf("sqlite database path is required")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("stat sqlite database: %w", err)
	}
	tempDir, artifactPath, err := createTempArtifact(task.TempDir, task.Name, strings.TrimPrefix(filepath.Ext(dbPath), "."))
	if err != nil {
		return nil, err
	}
	if filepath.Ext(artifactPath) == "." || filepath.Ext(artifactPath) == "" {
		artifactPath += ".sqlite"
	}
	if err := copyFile(dbPath, artifactPath); err != nil {
		return nil, err
	}
	writer.WriteLine("SQLite 备份文件已复制")
	return &RunResult{ArtifactPath: artifactPath, FileName: filepath.Base(artifactPath), TempDir: tempDir}, nil
}

func (r *SQLiteRunner) Restore(_ context.Context, task TaskSpec, artifactPath string, writer LogWriter) error {
	dbPath := filepath.Clean(strings.TrimSpace(task.Database.Path))
	if dbPath == "" {
		return fmt.Errorf("sqlite database path is required")
	}
	if err := copyFile(artifactPath, dbPath); err != nil {
		return err
	}
	writer.WriteLine("SQLite 数据库已恢复")
	return nil
}

func copyFile(sourcePath string, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}
	target, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create target file: %w", err)
	}
	defer target.Close()
	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("copy file content: %w", err)
	}
	return nil
}
