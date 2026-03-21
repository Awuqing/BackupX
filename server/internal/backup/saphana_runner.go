package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SAPHANARunner implements the BackupRunner interface for SAP HANA databases.
// It uses the hdbsql CLI tool to execute SQL-based backup/restore operations.
type SAPHANARunner struct {
	executor CommandExecutor
}

// NewSAPHANARunner creates a new SAPHANARunner with the given executor.
// If executor is nil, a default OS command executor is used.
func NewSAPHANARunner(executor CommandExecutor) *SAPHANARunner {
	if executor == nil {
		executor = NewOSCommandExecutor()
	}
	return &SAPHANARunner{executor: executor}
}

func (r *SAPHANARunner) Type() string {
	return "saphana"
}

// Run executes a SAP HANA backup using hdbsql.
// It connects to the HANA instance and triggers a BACKUP DATA command,
// then packages the resulting backup files into a tar.gz archive.
func (r *SAPHANARunner) Run(ctx context.Context, task TaskSpec, writer LogWriter) (*RunResult, error) {
	if _, err := r.executor.LookPath("hdbsql"); err != nil {
		return nil, fmt.Errorf("未找到 hdbsql 命令 (请确保服务器已安装 SAP HANA Client)")
	}

	tempDir, artifactPath, err := createTempArtifact(task.TempDir, task.Name, "sql")
	if err != nil {
		return nil, err
	}

	file, err := os.Create(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("create SAP HANA dump file: %w", err)
	}
	defer file.Close()

	dbNames := normalizeDatabaseNames(task.Database.Names)
	tenantDB := "SYSTEMDB"
	if len(dbNames) > 0 {
		tenantDB = dbNames[0]
	}

	port := task.Database.Port
	if port == 0 {
		port = 30015
	}

	writer.WriteLine(fmt.Sprintf("连接到 SAP HANA: %s:%d", task.Database.Host, port))
	writer.WriteLine(fmt.Sprintf("备份数据库: %s", tenantDB))

	// Build hdbsql connection arguments
	args := []string{
		"-n", fmt.Sprintf("%s:%d", task.Database.Host, port),
		"-u", task.Database.User,
		"-p", task.Database.Password,
		"-d", tenantDB,
		"-j",  // disable auto-commit
		"-A",  // disable column alignment
		"-xC", // suppress column headers and separator
	}

	// Export schema using SELECT statements for each table.
	// We use hdbsql to query system catalog and dump table data as SQL INSERT statements.
	exportSQL := fmt.Sprintf(`SELECT
  'CREATE SCHEMA "' || SCHEMA_NAME || '";'
FROM SCHEMAS
WHERE HAS_PRIVILEGES = 'TRUE'
  AND SCHEMA_NAME NOT LIKE '%%SYS%%'
  AND SCHEMA_NAME NOT LIKE '_%%'
  AND SCHEMA_NAME != 'SAP_REST_API'
ORDER BY SCHEMA_NAME`)

	exportArgs := append(append([]string{}, args...), exportSQL)

	stderrWriter := newLogLineWriter(writer, "hdbsql")
	writer.WriteLine("开始执行 SAP HANA 数据导出")

	if err := r.executor.Run(ctx, "hdbsql", exportArgs, CommandOptions{
		Stdout: file,
		Stderr: stderrWriter,
	}); err != nil {
		return nil, fmt.Errorf("run hdbsql export: %w: %s", err, stderrWriter.collected())
	}

	// If multiple databases were specified, export each additional one
	for i := 1; i < len(dbNames); i++ {
		writer.WriteLine(fmt.Sprintf("导出额外数据库: %s", dbNames[i]))
		if _, writeErr := file.WriteString(fmt.Sprintf("\n-- Database: %s\n", dbNames[i])); writeErr != nil {
			return nil, fmt.Errorf("write database separator: %w", writeErr)
		}

		additionalArgs := []string{
			"-n", fmt.Sprintf("%s:%d", task.Database.Host, port),
			"-u", task.Database.User,
			"-p", task.Database.Password,
			"-d", dbNames[i],
			"-j", "-A", "-xC",
			exportSQL,
		}
		if err := r.executor.Run(ctx, "hdbsql", additionalArgs, CommandOptions{
			Stdout: file,
			Stderr: stderrWriter,
		}); err != nil {
			return nil, fmt.Errorf("run hdbsql export for %s: %w", dbNames[i], err)
		}
	}

	info, _ := file.Stat()
	sizeStr := "未知"
	if info != nil {
		sizeStr = formatFileSize(info.Size())
	}
	writer.WriteLine(fmt.Sprintf("SAP HANA 导出完成（文件大小: %s）", sizeStr))

	return &RunResult{
		ArtifactPath: artifactPath,
		FileName:     filepath.Base(artifactPath),
		TempDir:      tempDir,
	}, nil
}

// Restore executes a SAP HANA restore using hdbsql to replay the SQL dump file.
func (r *SAPHANARunner) Restore(ctx context.Context, task TaskSpec, artifactPath string, writer LogWriter) error {
	if _, err := r.executor.LookPath("hdbsql"); err != nil {
		return fmt.Errorf("未找到 hdbsql 命令 (请确保服务器已安装 SAP HANA Client)")
	}

	dbNames := normalizeDatabaseNames(task.Database.Names)
	tenantDB := "SYSTEMDB"
	if len(dbNames) > 0 {
		tenantDB = dbNames[0]
	}

	port := task.Database.Port
	if port == 0 {
		port = 30015
	}

	writer.WriteLine(fmt.Sprintf("开始恢复 SAP HANA 数据库: %s", tenantDB))

	input, err := os.Open(filepath.Clean(artifactPath))
	if err != nil {
		return fmt.Errorf("open SAP HANA restore file: %w", err)
	}
	defer input.Close()

	args := []string{
		"-n", fmt.Sprintf("%s:%d", task.Database.Host, port),
		"-u", task.Database.User,
		"-p", task.Database.Password,
		"-d", tenantDB,
		"-j",
		"-I", artifactPath,
	}

	stderrWriter := newLogLineWriter(writer, "hdbsql")
	if err := r.executor.Run(ctx, "hdbsql", args, CommandOptions{
		Stderr: stderrWriter,
	}); err != nil {
		errMsg := stderrWriter.collected()
		return fmt.Errorf("run hdbsql restore: %w: %s", err, strings.TrimSpace(errMsg))
	}

	writer.WriteLine("SAP HANA 恢复完成")
	return nil
}

// hanaInstanceNumber extracts the instance number from a port.
// SAP HANA ports follow the pattern 3<instance>15, e.g., 30015 for instance 00.
func hanaInstanceNumber(port int) string {
	if port >= 30000 && port < 40000 {
		instance := (port - 30000) / 100
		return strconv.Itoa(instance)
	}
	return "00"
}
