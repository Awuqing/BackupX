package backup

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SAPHANARunner implements the BackupRunner interface for SAP HANA databases.
// It uses hdbsql to issue BACKUP DATA USING FILE commands for proper data-level
// backup (SAP best practice), rather than logical SQL export.
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

// Run executes a SAP HANA data-level backup using hdbsql + BACKUP DATA USING FILE.
// The backup files are written to a temporary directory, then packaged into a tar
// archive as the artifact for BackupX to compress/encrypt/upload.
//
// 支持以下增强（通过 task.Database 字段配置）：
//   - BackupLevel: full / incremental / differential
//   - BackupType:  data / log
//   - BackupChannels: 并行通道数（>1 时生成多路径 SQL）
//   - MaxRetries: hdbsql 执行失败的重试次数
func (r *SAPHANARunner) Run(ctx context.Context, task TaskSpec, writer LogWriter) (*RunResult, error) {
	if _, err := r.executor.LookPath("hdbsql"); err != nil {
		return nil, fmt.Errorf("未找到 hdbsql 命令 (请确保服务器已安装 SAP HANA Client)")
	}

	startedAt := task.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	// Create a temp directory for the tar artifact output.
	tempDir, artifactPath, err := createTempArtifact(task.TempDir, task.Name, "tar")
	if err != nil {
		return nil, err
	}

	// Create a sub-directory where HANA will write its backup data files.
	backupDir := filepath.Join(tempDir, "hana_data")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("create HANA backup directory: %w", err)
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

	backupLevel := normalizeBackupLevel(task.Database.BackupLevel)
	backupType := normalizeBackupType(task.Database.BackupType)
	channels := task.Database.BackupChannels
	if channels < 1 {
		channels = 1
	}
	maxRetries := task.Database.MaxRetries
	if maxRetries < 1 {
		maxRetries = 3
	}
	instance := task.Database.InstanceNumber
	if strings.TrimSpace(instance) == "" {
		instance = hanaInstanceNumber(port)
	}

	writer.WriteLine(fmt.Sprintf("连接到 SAP HANA: %s:%d (实例 %s)", task.Database.Host, port, instance))
	writer.WriteLine(fmt.Sprintf("备份数据库: %s", tenantDB))
	writer.WriteLine(fmt.Sprintf("备份配置: 类型=%s, 级别=%s, 通道数=%d, 最大重试=%d", backupType, backupLevel, channels, maxRetries))

	// Build backup prefix — HANA will create files like <prefix>_databackup_<N>_1.
	timestamp := startedAt.UTC().Format("20060102_150405")
	prefixes, err := buildBackupPrefixes(backupDir, tenantDB, timestamp, channels)
	if err != nil {
		return nil, err
	}

	// Build SQL based on backup type and level.
	backupSQL := buildBackupSQL(tenantDB, prefixes, backupType, backupLevel)
	writer.WriteLine(fmt.Sprintf("生成 SQL: %s", backupSQL))

	// Construct hdbsql connection arguments.
	args := buildHdbsqlArgs(task.Database.Host, port, task.Database.User, task.Database.Password, tenantDB, backupSQL)

	writer.WriteLine("开始执行 SAP HANA 备份命令")

	if err := r.runHdbsqlWithRetry(ctx, "hdbsql", args, maxRetries, writer); err != nil {
		return nil, fmt.Errorf("run hdbsql backup: %w", err)
	}

	writer.WriteLine("SAP HANA 备份命令执行完成，开始打包备份文件")

	// Package all generated backup files into a tar archive.
	if err := packageBackupFiles(backupDir, artifactPath, writer); err != nil {
		return nil, fmt.Errorf("package HANA backup files: %w", err)
	}

	info, _ := os.Stat(artifactPath)
	sizeStr := "未知"
	var fileSize int64
	if info != nil {
		fileSize = info.Size()
		sizeStr = formatFileSize(fileSize)
	}
	writer.WriteLine(fmt.Sprintf("SAP HANA 备份完成（归档大小: %s）", sizeStr))

	return &RunResult{
		ArtifactPath: artifactPath,
		FileName:     filepath.Base(artifactPath),
		TempDir:      tempDir,
		Size:         fileSize,
		StorageKey:   BuildStorageKey("saphana", startedAt, filepath.Base(artifactPath)),
	}, nil
}

// Restore executes a SAP HANA restore using RECOVER DATA USING FILE.
// It extracts the tar archive to get the original backup files, then issues
// the recovery SQL command via hdbsql.
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

	// Extract the tar archive to a temporary directory.
	restoreDir, err := os.MkdirTemp("", "backupx-hana-restore-*")
	if err != nil {
		return fmt.Errorf("create restore temp dir: %w", err)
	}
	defer os.RemoveAll(restoreDir)

	if err := extractTarArchive(artifactPath, restoreDir); err != nil {
		return fmt.Errorf("extract HANA backup tar: %w", err)
	}

	// Find the backup prefix by locating backup data files.
	prefix, err := findBackupPrefix(restoreDir)
	if err != nil {
		return fmt.Errorf("find backup prefix: %w", err)
	}

	writer.WriteLine(fmt.Sprintf("找到备份前缀: %s", filepath.Base(prefix)))

	// Build RECOVER DATA SQL.
	recoverSQL := fmt.Sprintf(`RECOVER DATA USING FILE ('%s') CLEAR LOG`, prefix)
	if strings.ToUpper(tenantDB) != "SYSTEMDB" {
		recoverSQL = fmt.Sprintf(`RECOVER DATA FOR %s USING FILE ('%s') CLEAR LOG`, tenantDB, prefix)
	}

	args := buildHdbsqlArgs(task.Database.Host, port, task.Database.User, task.Database.Password, tenantDB, recoverSQL)

	maxRetries := task.Database.MaxRetries
	if maxRetries < 1 {
		maxRetries = 3
	}
	if err := r.runHdbsqlWithRetry(ctx, "hdbsql", args, maxRetries, writer); err != nil {
		return fmt.Errorf("run hdbsql RECOVER DATA: %w", err)
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

// normalizeBackupLevel 规范化备份级别值，无效或空值默认为 "full"。
func normalizeBackupLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "incremental":
		return "incremental"
	case "differential":
		return "differential"
	default:
		return "full"
	}
}

// normalizeBackupType 规范化备份类型，无效或空值默认为 "data"。
func normalizeBackupType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "log":
		return "log"
	default:
		return "data"
	}
}

// buildBackupPrefixes 为每个并行通道生成独立子目录和路径前缀。
// 当 channels=1 时返回单个直接位于 backupDir 下的前缀；
// 当 channels>1 时为每个通道创建 chan_N/ 子目录。
func buildBackupPrefixes(backupDir, tenantDB, timestamp string, channels int) ([]string, error) {
	tenantLower := strings.ToLower(tenantDB)
	if channels <= 1 {
		return []string{filepath.Join(backupDir, fmt.Sprintf("hana_%s_%s", tenantLower, timestamp))}, nil
	}
	prefixes := make([]string, 0, channels)
	for i := 0; i < channels; i++ {
		chanDir := filepath.Join(backupDir, fmt.Sprintf("chan_%d", i))
		if err := os.MkdirAll(chanDir, 0o755); err != nil {
			return nil, fmt.Errorf("create channel %d dir: %w", i, err)
		}
		prefixes = append(prefixes, filepath.Join(chanDir, fmt.Sprintf("hana_%s_%s", tenantLower, timestamp)))
	}
	return prefixes, nil
}

// buildBackupSQL 根据备份类型和级别构建 SAP HANA BACKUP SQL 语句。
//
// 支持的语法：
//
//	全量数据备份:    BACKUP DATA [FOR <tenant>] USING FILE ('p1' [, 'p2', ...])
//	增量数据备份:    BACKUP DATA [FOR <tenant>] INCREMENTAL USING FILE ('...')
//	差异数据备份:    BACKUP DATA [FOR <tenant>] DIFFERENTIAL USING FILE ('...')
//	日志备份:        BACKUP LOG [FOR <tenant>] USING FILE ('...')
func buildBackupSQL(tenantDB string, prefixes []string, backupType, backupLevel string) string {
	tenantClause := ""
	if strings.ToUpper(tenantDB) != "SYSTEMDB" {
		tenantClause = fmt.Sprintf(" FOR %s", tenantDB)
	}

	// 多路径以 'p1', 'p2', ... 拼接（HANA 多通道并行语法）
	quoted := make([]string, len(prefixes))
	for i, p := range prefixes {
		quoted[i] = fmt.Sprintf("'%s'", p)
	}
	pathClause := strings.Join(quoted, ", ")

	if backupType == "log" {
		// LOG 备份不支持 INCREMENTAL/DIFFERENTIAL 关键字
		return fmt.Sprintf("BACKUP LOG%s USING FILE (%s)", tenantClause, pathClause)
	}

	levelClause := ""
	switch backupLevel {
	case "incremental":
		levelClause = " INCREMENTAL"
	case "differential":
		levelClause = " DIFFERENTIAL"
	}
	return fmt.Sprintf("BACKUP DATA%s%s USING FILE (%s)", tenantClause, levelClause, pathClause)
}

// runHdbsqlWithRetry 执行 hdbsql 命令并在失败时按指数退避重试。
// 退避公式：5s × attempt²，并在 ctx 取消时立即返回。
func (r *SAPHANARunner) runHdbsqlWithRetry(ctx context.Context, name string, args []string, maxAttempts int, writer LogWriter) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(attempt*attempt) * 5 * time.Second
			writer.WriteLine(fmt.Sprintf("hdbsql 第 %d 次重试（等待 %s）", attempt, backoff))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		stderrWriter := newLogLineWriter(writer, "hdbsql")
		err := r.executor.Run(ctx, name, args, CommandOptions{Stderr: stderrWriter})
		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("%w: %s", err, strings.TrimSpace(stderrWriter.collected()))
		writer.WriteLine(fmt.Sprintf("hdbsql 执行失败（第 %d/%d 次）: %v", attempt, maxAttempts, lastErr))
	}
	return lastErr
}

// buildHdbsqlArgs constructs the common hdbsql CLI arguments.
func buildHdbsqlArgs(host string, port int, user, password, database, sql string) []string {
	return []string{
		"-n", fmt.Sprintf("%s:%d", host, port),
		"-u", user,
		"-p", password,
		"-d", database,
		"-j",  // disable auto-commit
		"-A",  // disable column alignment
		"-xC", // suppress column headers and separator
		sql,
	}
}

// packageBackupFiles creates a tar archive from all files in the given directory.
func packageBackupFiles(sourceDir, targetPath string, writer LogWriter) error {
	file, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create tar file: %w", err)
	}
	defer file.Close()

	tw := tar.NewWriter(file)
	defer tw.Close()

	fileCount := 0
	walkErr := filepath.Walk(sourceDir, func(currentPath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if currentPath == sourceDir {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, currentPath)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			f, err := os.Open(currentPath)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.CopyN(tw, f, info.Size()); err != nil && err != io.EOF {
				return err
			}
			fileCount++
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	if fileCount == 0 {
		return fmt.Errorf("HANA 备份目录中未找到任何备份文件")
	}

	writer.WriteLine(fmt.Sprintf("已打包 %d 个备份文件", fileCount))
	return nil
}

// extractTarArchive extracts a tar archive to the given directory.
func extractTarArchive(tarPath, targetDir string) error {
	f, err := os.Open(filepath.Clean(tarPath))
	if err != nil {
		return err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		targetPath := filepath.Join(targetDir, filepath.FromSlash(filepath.Clean(header.Name)))
		// Guard against path traversal.
		if !strings.HasPrefix(targetPath, filepath.Clean(targetDir)+string(filepath.Separator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			outFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}

// findBackupPrefix locates the backup prefix by scanning for HANA backup data files.
// HANA creates files like <prefix>_databackup_0_1, <prefix>_databackup_1_1, etc.
func findBackupPrefix(dir string) (string, error) {
	var prefix string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		name := info.Name()
		if idx := strings.Index(name, "_databackup_"); idx > 0 {
			prefix = filepath.Join(filepath.Dir(path), name[:idx])
			return filepath.SkipAll
		}
		// Also check for the complete backup file pattern without _databackup_
		if strings.HasPrefix(name, "hana_") {
			prefix = filepath.Join(filepath.Dir(path), strings.TrimSuffix(name, filepath.Ext(name)))
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return "", err
	}
	if prefix == "" {
		return "", fmt.Errorf("未在归档中找到 HANA 备份数据文件")
	}
	return prefix, nil
}
