package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type MySQLRunner struct {
	executor CommandExecutor
}

func NewMySQLRunner(executor CommandExecutor) *MySQLRunner {
	if executor == nil {
		executor = NewOSCommandExecutor()
	}
	return &MySQLRunner{executor: executor}
}

func (r *MySQLRunner) Type() string {
	return "mysql"
}

func (r *MySQLRunner) Run(ctx context.Context, task TaskSpec, writer LogWriter) (*RunResult, error) {
	if task.MySQLPITR {
		return r.runPITR(ctx, task, writer)
	}
	if _, err := r.executor.LookPath("mysqldump"); err != nil {
		return nil, fmt.Errorf("未找到 mysqldump 命令 (请确保服务器已安装 mysql-client 或 mariadb-client)")
	}
	startedAt := task.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	tempDir, err := CreateTaskTempDir(task.Name, startedAt)
	if err != nil {
		return nil, err
	}
	fileName := BuildArtifactName(task.Name, startedAt, "sql")
	artifactPath := filepath.Join(tempDir, fileName)
	file, err := os.Create(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("create mysql dump file: %w", err)
	}
	defer file.Close()
	dbNames := normalizeDatabaseNames(task.Database.Names)
	if len(dbNames) == 0 {
		return nil, fmt.Errorf("mysql database names are required")
	}
	args := []string{
		"--host", task.Database.Host,
		"--port", strconv.Itoa(task.Database.Port),
		"--user", task.Database.User,
		"--single-transaction",
		"--quick",
		"--routines",
		"--triggers",
		"--events",
		"--no-tablespaces",
		"--net-buffer-length=32768",
		"--databases",
	}
	args = append(args, dbNames...)

	writer.WriteLine(fmt.Sprintf("连接到 MySQL: %s:%d", task.Database.Host, task.Database.Port))
	writer.WriteLine(fmt.Sprintf("备份数据库: %s", strings.Join(dbNames, ", ")))

	stderrWriter := newLogLineWriter(writer, "mysqldump")
	writer.WriteLine("开始执行 mysqldump")
	runErr := r.executor.Run(ctx, "mysqldump", args, CommandOptions{Stdout: file, Stderr: stderrWriter, Env: mysqlEnv(task.Database.Password)})
	stderrWriter.Flush()
	if runErr != nil {
		return nil, fmt.Errorf("run mysqldump: %w: %s", runErr, stderrWriter.collected())
	}
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat mysql dump file: %w", err)
	}
	writer.WriteLine(fmt.Sprintf("MySQL 导出完成（文件大小: %s）", formatFileSize(info.Size())))
	return &RunResult{ArtifactPath: artifactPath, FileName: fileName, TempDir: tempDir, Size: info.Size(), StorageKey: BuildStorageKey("mysql", startedAt, fileName)}, nil
}

func (r *MySQLRunner) Restore(ctx context.Context, task TaskSpec, artifactPath string, writer LogWriter) error {
	if strings.HasSuffix(artifactPath, ".pitr.tar") {
		prepared, err := r.PrepareRestore(ctx, task, artifactPath, writer)
		if err != nil {
			return err
		}
		return r.restoreSQL(ctx, task, prepared, writer)
	}
	return r.restoreSQL(ctx, task, artifactPath, writer)
}

func (r *MySQLRunner) restoreSQL(ctx context.Context, task TaskSpec, artifactPath string, writer LogWriter) error {
	if _, err := r.executor.LookPath("mysql"); err != nil {
		return fmt.Errorf("未找到 mysql 命令 (请确保服务器已安装 mysql-client 或 mariadb-client)")
	}
	input, err := os.Open(filepath.Clean(artifactPath))
	if err != nil {
		return fmt.Errorf("open mysql restore file: %w", err)
	}
	defer input.Close()
	stderr := &bytes.Buffer{}
	args := []string{"--host", task.Database.Host, "--port", strconv.Itoa(task.Database.Port), "--user", task.Database.User}
	writer.WriteLine("开始执行 mysql 恢复")
	if err := r.executor.Run(ctx, "mysql", args, CommandOptions{Stdin: input, Stderr: stderr, Env: mysqlEnv(task.Database.Password)}); err != nil {
		return fmt.Errorf("run mysql restore: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	writer.WriteLine("MySQL 恢复完成")
	return nil
}

func mysqlEnv(password string) []string {
	if strings.TrimSpace(password) == "" {
		return nil
	}
	return []string{"MYSQL_PWD=" + password}
}

// logLineWriter streams each line of output to a LogWriter in real-time.
type logLineWriter struct {
	writer  LogWriter
	prefix  string
	pending []byte
	output  []byte
}

func newLogLineWriter(w LogWriter, prefix string) *logLineWriter {
	return &logLineWriter{writer: w, prefix: prefix}
}

func (w *logLineWriter) Write(p []byte) (int, error) {
	w.output = append(w.output, p...)
	w.pending = append(w.pending, p...)
	consumed := 0
	for {
		newline := bytes.IndexByte(w.pending[consumed:], '\n')
		if newline < 0 {
			break
		}
		end := consumed + newline
		w.emit(w.pending[consumed:end])
		consumed = end + 1
	}
	if consumed > 0 {
		copy(w.pending, w.pending[consumed:])
		w.pending = w.pending[:len(w.pending)-consumed]
	}
	return len(p), nil
}

// Flush emits the final unterminated line. It is safe to call more than once.
func (w *logLineWriter) Flush() {
	if len(w.pending) == 0 {
		return
	}
	w.emit(w.pending)
	w.pending = w.pending[:0]
}

func (w *logLineWriter) emit(raw []byte) {
	line := strings.TrimSpace(string(raw))
	if line != "" {
		w.writer.WriteLine(fmt.Sprintf("[%s] %s", w.prefix, line))
	}
}

func (w *logLineWriter) collected() string {
	return strings.TrimSpace(string(w.output))
}

func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}
