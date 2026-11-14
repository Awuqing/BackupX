package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MySQLCheckpoint describes a cumulative binlog archive rooted at one full dump.
// All database names are included: database-level binlog filtering is unsafe for
// cross-database transactions and statements using qualified table names.
type MySQLCheckpoint struct {
	ServerUUID    string    `json:"serverUuid"`
	Source        string    `json:"source"`
	StartFile     string    `json:"startFile"`
	StartPosition uint64    `json:"startPosition"`
	EndFile       string    `json:"endFile"`
	EndPosition   uint64    `json:"endPosition"`
	CapturedAt    time.Time `json:"capturedAt"`
	BaselineAt    time.Time `json:"baselineAt"`
	Files         []string  `json:"files,omitempty"`
	Full          bool      `json:"full"`
}

var binlogNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-][a-zA-Z0-9_.-]*\.[0-9]+$`)
var dumpPositionPattern = regexp.MustCompile(`(?m)^-- CHANGE (?:REPLICATION SOURCE|MASTER) TO (?:SOURCE|MASTER)_LOG_FILE='([^']+)', (?:SOURCE|MASTER)_LOG_POS=([0-9]+);`)

func MySQLPITREnabled(extra string) bool {
	var options struct {
		Enabled bool `json:"mysqlPitr"`
	}
	return json.Unmarshal([]byte(extra), &options) == nil && options.Enabled
}

func MySQLFullInterval(extra string) time.Duration {
	var options struct {
		Hours int `json:"fullIntervalHours"`
	}
	if json.Unmarshal([]byte(extra), &options) != nil || options.Hours <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(options.Hours) * time.Hour
}

func MySQLSource(db DatabaseSpec) string {
	return fmt.Sprintf("%s:%d", strings.ToLower(strings.TrimSpace(db.Host)), db.Port)
}

func (c *MySQLCheckpoint) Validate() error {
	if c == nil || c.ServerUUID == "" || c.Source == "" || !binlogNamePattern.MatchString(c.StartFile) || !binlogNamePattern.MatchString(c.EndFile) || c.StartPosition < 4 || c.EndPosition < 4 || c.CapturedAt.IsZero() || c.BaselineAt.IsZero() || c.CapturedAt.Before(c.BaselineAt) {
		return errors.New("invalid MySQL PITR checkpoint")
	}
	if !c.Full {
		if len(c.Files) == 0 || c.Files[0] != c.StartFile || c.Files[len(c.Files)-1] != c.EndFile {
			return errors.New("incomplete MySQL binlog archive")
		}
		for i, name := range c.Files {
			if !binlogNamePattern.MatchString(name) {
				return errors.New("invalid binlog file name")
			}
			if i > 0 && !consecutiveBinlog(c.Files[i-1], name) {
				return errors.New("gap in MySQL binlog sequence")
			}
		}
		if c.StartFile == c.EndFile && c.EndPosition < c.StartPosition {
			return errors.New("MySQL binlog position moved backwards")
		}
	}
	return nil
}

func consecutiveBinlog(previous, next string) bool {
	a, b := strings.LastIndexByte(previous, '.'), strings.LastIndexByte(next, '.')
	if a < 0 || b < 0 || previous[:a] != next[:b] {
		return false
	}
	x, e1 := strconv.ParseUint(previous[a+1:], 10, 64)
	y, e2 := strconv.ParseUint(next[b+1:], 10, 64)
	return e1 == nil && e2 == nil && y > x && y-x == 1
}

func mysqlConnectionArgs(db DatabaseSpec) []string {
	return []string{"--host", db.Host, "--port", strconv.Itoa(db.Port), "--user", db.User}
}

func (r *MySQLRunner) query(ctx context.Context, db DatabaseSpec, sql string) (string, error) {
	var out, stderr bytes.Buffer
	args := append(mysqlConnectionArgs(db), "--batch", "--skip-column-names", "--execute", sql)
	err := r.executor.Run(ctx, "mysql", args, CommandOptions{Stdout: &out, Stderr: &stderr, Env: mysqlEnv(db.Password)})
	if err != nil {
		return "", fmt.Errorf("query MySQL metadata: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func (r *MySQLRunner) checkpoint(ctx context.Context, db DatabaseSpec) (*MySQLCheckpoint, error) {
	// Capture time before the log position, so the archive covers the entire advertised window.
	meta, err := r.query(ctx, db, "SELECT @@server_uuid, UTC_TIMESTAMP(), @@global.log_bin")
	if err != nil {
		return nil, err
	}
	fields := strings.Split(meta, "\t")
	if len(fields) != 3 || fields[2] != "1" {
		return nil, errors.New("MySQL PITR requires MySQL 8.0+ with binary logging enabled")
	}
	captured, err := time.ParseInLocation("2006-01-02 15:04:05", fields[1], time.UTC)
	if err != nil {
		return nil, err
	}
	status, err := r.query(ctx, db, "SHOW BINARY LOG STATUS")
	if err != nil {
		status, err = r.query(ctx, db, "SHOW MASTER STATUS")
	}
	if err != nil {
		return nil, err
	}
	pos := strings.Fields(status)
	if len(pos) < 2 || !binlogNamePattern.MatchString(pos[0]) {
		return nil, errors.New("missing MySQL binary log status")
	}
	offset, err := strconv.ParseUint(pos[1], 10, 64)
	if err != nil || offset < 4 {
		return nil, errors.New("invalid MySQL binary log position")
	}
	return &MySQLCheckpoint{ServerUUID: fields[0], Source: MySQLSource(db), EndFile: pos[0], EndPosition: offset, CapturedAt: captured}, nil
}

func (r *MySQLRunner) runPITR(ctx context.Context, task TaskSpec, writer LogWriter) (result *RunResult, err error) {
	if task.StartedAt.IsZero() {
		task.StartedAt = time.Now().UTC()
	}
	for _, tool := range []string{"mysql", "mysqldump", "mysqlbinlog"} {
		if _, err := r.executor.LookPath(tool); err != nil {
			return nil, fmt.Errorf("MySQL PITR requires %s (MySQL 8.0+ client tools)", tool)
		}
	}
	names := normalizeDatabaseNames(task.Database.Names)
	if len(names) != 1 || names[0] != "*" {
		return nil, errors.New("MySQL PITR requires database name * (whole instance)")
	}
	var dir string
	if task.TempDir != "" {
		if err := os.MkdirAll(task.TempDir, 0700); err != nil {
			return nil, err
		}
		dir, err = os.MkdirTemp(task.TempDir, "mysql-pitr-*")
	} else {
		dir, err = CreateTaskTempDir(task.Name, task.StartedAt)
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(dir))
		}
	}()
	dataDir := filepath.Join(dir, "pitr")
	if err := os.Mkdir(dataDir, 0700); err != nil {
		return nil, err
	}
	checkpoint, err := r.checkpoint(ctx, task.Database)
	if err != nil {
		return nil, err
	}
	base := task.MySQLBase
	if base == nil {
		dumpPath := filepath.Join(dataDir, "dump.sql")
		output, err := os.OpenFile(dumpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return nil, err
		}
		args := append(mysqlConnectionArgs(task.Database), "--single-transaction", "--quick", "--routines", "--triggers", "--events", "--no-tablespaces", "--source-data=2", "--set-gtid-purged=OFF", "--all-databases")
		stderr := newLogLineWriter(writer, "mysqldump")
		writer.WriteLine("创建 MySQL PITR 全实例基线（仅 InnoDB；期间请勿执行 DDL）")
		runErr := r.executor.Run(ctx, "mysqldump", args, CommandOptions{Stdout: output, Stderr: stderr, Env: mysqlEnv(task.Database.Password)})
		stderr.Flush()
		if err := errors.Join(runErr, output.Close()); err != nil {
			return nil, fmt.Errorf("create PITR baseline: %w: %s", err, stderr.collected())
		}
		// The coordinate is near the header; avoid reading the complete database into memory.
		input, err := os.Open(dumpPath)
		if err != nil {
			return nil, err
		}
		header, readErr := io.ReadAll(io.LimitReader(input, 1<<20))
		if err := errors.Join(readErr, input.Close()); err != nil {
			return nil, err
		}
		match := dumpPositionPattern.FindSubmatch(header)
		if len(match) != 3 || !binlogNamePattern.Match(match[1]) {
			return nil, errors.New("mysqldump did not report a binary log coordinate")
		}
		position, err := strconv.ParseUint(string(match[2]), 10, 64)
		if err != nil {
			return nil, err
		}
		beforeUUID := checkpoint.ServerUUID
		checkpoint, err = r.checkpoint(ctx, task.Database)
		if err != nil {
			return nil, err
		}
		if checkpoint.ServerUUID != beforeUUID {
			return nil, errors.New("MySQL server changed during full backup")
		}
		checkpoint.StartFile, checkpoint.StartPosition = string(match[1]), position
		checkpoint.BaselineAt = checkpoint.CapturedAt
		checkpoint.Full = true
	} else {
		if err := base.Validate(); err != nil {
			return nil, err
		}
		if !base.Full || base.Source != checkpoint.Source || base.ServerUUID != checkpoint.ServerUUID || checkpoint.CapturedAt.Before(base.CapturedAt) {
			return nil, errors.New("MySQL source changed; create a new full baseline")
		}
		checkpoint.StartFile, checkpoint.StartPosition = base.StartFile, base.StartPosition
		checkpoint.BaselineAt = base.BaselineAt
		logs, err := r.query(ctx, task.Database, "SHOW BINARY LOGS")
		if err != nil {
			return nil, err
		}
		started := false
		expectedSizes := map[string]uint64{}
		for _, line := range strings.Split(logs, "\n") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return nil, errors.New("invalid MySQL binary log list")
			}
			name := fields[0]
			if name == base.StartFile {
				started = true
			}
			if !started {
				continue
			}
			expectedSize, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil || expectedSize < 4 {
				return nil, errors.New("invalid binary log size")
			}
			expectedSizes[name] = expectedSize
			checkpoint.Files = append(checkpoint.Files, name)
			if name == checkpoint.EndFile {
				break
			}
		}
		if err := checkpoint.Validate(); err != nil {
			return nil, fmt.Errorf("binlogs expired or incomplete; no recoverable archive created: %w", err)
		}
		for _, name := range checkpoint.Files {
			args := append(mysqlConnectionArgs(task.Database), "--read-from-remote-server", "--raw", "--verify-binlog-checksum", "--result-file="+dataDir+string(filepath.Separator), name)
			stderr := newLogLineWriter(writer, "mysqlbinlog")
			runErr := r.executor.Run(ctx, "mysqlbinlog", args, CommandOptions{Stderr: stderr, Env: mysqlEnv(task.Database.Password)})
			stderr.Flush()
			if runErr != nil {
				return nil, fmt.Errorf("archive binlog %s: %w: %s", name, runErr, stderr.collected())
			}
			info, err := os.Stat(filepath.Join(dataDir, name))
			if err != nil {
				return nil, err
			}
			if (name == checkpoint.EndFile && uint64(info.Size()) < checkpoint.EndPosition) || (name != checkpoint.EndFile && uint64(info.Size()) != expectedSizes[name]) {
				return nil, errors.New("downloaded binlog is shorter than captured position")
			}
		}
		writer.WriteLine(fmt.Sprintf("已归档基线以来的 %d 个 binlog；恢复范围截至 %s UTC", len(checkpoint.Files), checkpoint.CapturedAt.Format(time.DateTime)))
	}
	if err := checkpoint.Validate(); err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(checkpoint)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dataDir, "checkpoint.json"), metadata, 0600); err != nil {
		return nil, err
	}
	name := BuildArtifactName(task.Name, task.StartedAt, "pitr.tar")
	artifact := filepath.Join(dir, name)
	if err := packageBackupFiles(dataDir, artifact, writer); err != nil {
		return nil, err
	}
	info, err := os.Stat(artifact)
	if err != nil {
		return nil, err
	}
	return &RunResult{ArtifactPath: artifact, FileName: name, TempDir: dir, Size: info.Size(), MySQLCheckpoint: checkpoint}, nil
}

// PrepareRestore verifies and decodes all PITR data before the caller imports a
// baseline. The caller owns the artifact's parent temporary directory.
func (r *MySQLRunner) PrepareRestore(ctx context.Context, task TaskSpec, artifact string, writer LogWriter) (string, error) {
	if !strings.HasSuffix(artifact, ".pitr.tar") {
		return artifact, nil
	}
	dir, err := os.MkdirTemp(filepath.Dir(artifact), "mysql-pitr-restore-*")
	if err != nil {
		return "", err
	}
	if err := extractTarArchive(artifact, dir); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "checkpoint.json"))
	if err != nil {
		return "", err
	}
	var checkpoint MySQLCheckpoint
	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		return "", err
	}
	if err := checkpoint.Validate(); err != nil {
		return "", err
	}
	if !task.RestoreTime.IsZero() && (task.RestoreTime.Before(checkpoint.BaselineAt) || (!checkpoint.Full && !task.RestoreTime.Before(checkpoint.CapturedAt))) {
		return "", errors.New("requested restore time is outside the archived MySQL window (end exclusive)")
	}
	if checkpoint.Full {
		return filepath.Join(dir, "dump.sql"), nil
	}
	if _, err := r.executor.LookPath("mysqlbinlog"); err != nil {
		return "", err
	}
	args := []string{"--verify-binlog-checksum", "--skip-gtids", "--disable-log-bin", "--start-position=" + strconv.FormatUint(checkpoint.StartPosition, 10), "--stop-position=" + strconv.FormatUint(checkpoint.EndPosition, 10)}
	if !task.RestoreTime.IsZero() {
		args = append(args, "--stop-datetime="+task.RestoreTime.UTC().Format(time.DateTime))
	}
	for _, name := range checkpoint.Files {
		args = append(args, filepath.Join(dir, name))
	}
	// Decode completely before touching the database, so corrupt or missing logs cannot be partially replayed.
	sqlPath := filepath.Join(dir, "replay.sql")
	output, err := os.OpenFile(sqlPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	runErr := r.executor.Run(ctx, "mysqlbinlog", args, CommandOptions{Stdout: output, Stderr: &stderr, Env: []string{"TZ=UTC"}})
	if err := errors.Join(runErr, output.Close()); err != nil {
		return "", fmt.Errorf("decode MySQL binlog: %w: %s", err, stderr.String())
	}
	return sqlPath, nil
}
