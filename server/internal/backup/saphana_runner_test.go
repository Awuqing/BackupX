package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSAPHANARunnerRun_BackupDataCommand(t *testing.T) {
	var capturedArgs []string
	executor := &fakeCommandExecutor{
		runFunc: func(name string, args []string, options CommandOptions) error {
			capturedArgs = append([]string{}, args...)
			// Simulate HANA creating backup data files in the directory from the SQL.
			// Parse the backup prefix from the SQL argument (last arg).
			sql := args[len(args)-1]
			// Extract path from: BACKUP DATA USING FILE ('/path/to/hana_systemdb_...')
			startIdx := strings.Index(sql, "('") + 2
			endIdx := strings.Index(sql, "')")
			if startIdx > 1 && endIdx > startIdx {
				prefix := sql[startIdx:endIdx]
				dir := filepath.Dir(prefix)
				_ = os.MkdirAll(dir, 0o755)
				// Create fake backup data files that HANA would produce.
				_ = os.WriteFile(prefix+"_databackup_0_1", []byte("fake backup data volume 0"), 0o644)
				_ = os.WriteFile(prefix+"_databackup_1_1", []byte("fake backup data volume 1"), 0o644)
			}
			return nil
		},
	}

	runner := NewSAPHANARunner(executor)
	result, err := runner.Run(context.Background(), TaskSpec{
		Name: "hana-daily",
		Type: "saphana",
		Database: DatabaseSpec{
			Host:     "10.0.0.1",
			Port:     30015,
			User:     "SYSTEM",
			Password: "secret",
			Names:    []string{"SYSTEMDB"},
		},
	}, NopLogWriter{})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Verify hdbsql was called with the correct connection args.
	if len(capturedArgs) == 0 {
		t.Fatal("expected hdbsql args to be captured")
	}

	// Check host:port
	foundHost := false
	for i, arg := range capturedArgs {
		if arg == "-n" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "10.0.0.1:30015" {
			foundHost = true
		}
	}
	if !foundHost {
		t.Fatalf("expected host:port 10.0.0.1:30015 in args, got: %v", capturedArgs)
	}

	// Verify the SQL contains BACKUP DATA USING FILE.
	lastArg := capturedArgs[len(capturedArgs)-1]
	if !strings.Contains(lastArg, "BACKUP DATA USING FILE") {
		t.Fatalf("expected BACKUP DATA USING FILE in SQL, got: %s", lastArg)
	}

	// Verify artifact is a tar file.
	if !strings.HasSuffix(result.ArtifactPath, ".tar") {
		t.Fatalf("expected .tar artifact, got: %s", result.ArtifactPath)
	}

	// Verify artifact file exists and has content.
	info, err := os.Stat(result.ArtifactPath)
	if err != nil {
		t.Fatalf("artifact file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("artifact tar file is empty")
	}

	// Cleanup.
	os.RemoveAll(result.TempDir)
}

func TestSAPHANARunnerRun_TenantDatabase(t *testing.T) {
	var capturedSQL string
	executor := &fakeCommandExecutor{
		runFunc: func(name string, args []string, options CommandOptions) error {
			capturedSQL = args[len(args)-1]
			// Simulate HANA creating backup files.
			startIdx := strings.Index(capturedSQL, "('") + 2
			endIdx := strings.Index(capturedSQL, "')")
			if startIdx > 1 && endIdx > startIdx {
				prefix := capturedSQL[startIdx:endIdx]
				_ = os.MkdirAll(filepath.Dir(prefix), 0o755)
				_ = os.WriteFile(prefix+"_databackup_0_1", []byte("data"), 0o644)
			}
			return nil
		},
	}

	runner := NewSAPHANARunner(executor)
	result, err := runner.Run(context.Background(), TaskSpec{
		Name: "hana-tenant",
		Type: "saphana",
		Database: DatabaseSpec{
			Host:     "10.0.0.1",
			Port:     30015,
			User:     "SYSTEM",
			Password: "secret",
			Names:    []string{"HDB"},
		},
	}, NopLogWriter{})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	defer os.RemoveAll(result.TempDir)

	// For tenant databases, the SQL should use BACKUP DATA FOR <tenant>.
	if !strings.Contains(capturedSQL, "BACKUP DATA FOR HDB USING FILE") {
		t.Fatalf("expected BACKUP DATA FOR HDB in SQL, got: %s", capturedSQL)
	}
}

func TestSAPHANARunnerRun_DefaultPort(t *testing.T) {
	var capturedArgs []string
	executor := &fakeCommandExecutor{
		runFunc: func(name string, args []string, options CommandOptions) error {
			capturedArgs = append([]string{}, args...)
			sql := args[len(args)-1]
			startIdx := strings.Index(sql, "('") + 2
			endIdx := strings.Index(sql, "')")
			if startIdx > 1 && endIdx > startIdx {
				prefix := sql[startIdx:endIdx]
				_ = os.MkdirAll(filepath.Dir(prefix), 0o755)
				_ = os.WriteFile(prefix+"_databackup_0_1", []byte("data"), 0o644)
			}
			return nil
		},
	}

	runner := NewSAPHANARunner(executor)
	result, err := runner.Run(context.Background(), TaskSpec{
		Name: "hana-default-port",
		Type: "saphana",
		Database: DatabaseSpec{
			Host:     "localhost",
			Port:     0, // Should default to 30015
			User:     "SYSTEM",
			Password: "secret",
		},
	}, NopLogWriter{})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	defer os.RemoveAll(result.TempDir)

	// Verify default port 30015 was used.
	for i, arg := range capturedArgs {
		if arg == "-n" && i+1 < len(capturedArgs) {
			if !strings.HasSuffix(capturedArgs[i+1], ":30015") {
				t.Fatalf("expected default port 30015, got: %s", capturedArgs[i+1])
			}
		}
	}
}

func TestSAPHANARunnerRun_LookPathError(t *testing.T) {
	runner := NewSAPHANARunner(&fakeCommandExecutor{lookupErr: errors.New("not found")})
	_, err := runner.Run(context.Background(), TaskSpec{
		Name: "hana-missing",
		Type: "saphana",
		Database: DatabaseSpec{
			Host: "10.0.0.1", Port: 30015, User: "SYSTEM", Password: "secret",
		},
	}, NopLogWriter{})
	if err == nil {
		t.Fatal("expected error when hdbsql is missing")
	}
	if !strings.Contains(err.Error(), "hdbsql") {
		t.Fatalf("error should mention hdbsql, got: %v", err)
	}
}

func TestSAPHANARunnerRestore_RecoverDataCommand(t *testing.T) {
	// First, create a fake tar archive with a backup data file.
	tarDir := t.TempDir()
	dataDir := filepath.Join(tarDir, "hana_data")
	_ = os.MkdirAll(dataDir, 0o755)
	prefix := filepath.Join(dataDir, "hana_systemdb_20260324_120000")
	_ = os.WriteFile(prefix+"_databackup_0_1", []byte("backup data"), 0o644)

	// Create the tar.
	tarPath := filepath.Join(tarDir, "backup.tar")
	if err := packageBackupFiles(dataDir, tarPath, NopLogWriter{}); err != nil {
		t.Fatalf("failed to create test tar: %v", err)
	}

	var capturedSQL string
	executor := &fakeCommandExecutor{
		runFunc: func(name string, args []string, options CommandOptions) error {
			capturedSQL = args[len(args)-1]
			return nil
		},
	}

	runner := NewSAPHANARunner(executor)
	err := runner.Restore(context.Background(), TaskSpec{
		Name: "hana-restore",
		Type: "saphana",
		Database: DatabaseSpec{
			Host: "10.0.0.1", Port: 30015, User: "SYSTEM", Password: "secret",
			Names: []string{"SYSTEMDB"},
		},
	}, tarPath, NopLogWriter{})

	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}

	if !strings.Contains(capturedSQL, "RECOVER DATA USING FILE") {
		t.Fatalf("expected RECOVER DATA USING FILE in SQL, got: %s", capturedSQL)
	}
	if !strings.Contains(capturedSQL, "CLEAR LOG") {
		t.Fatalf("expected CLEAR LOG in SQL, got: %s", capturedSQL)
	}
}

func TestSAPHANARunnerRestore_TenantRecoverCommand(t *testing.T) {
	tarDir := t.TempDir()
	dataDir := filepath.Join(tarDir, "data")
	_ = os.MkdirAll(dataDir, 0o755)
	_ = os.WriteFile(filepath.Join(dataDir, "hana_hdb_20260324_120000_databackup_0_1"), []byte("data"), 0o644)

	tarPath := filepath.Join(tarDir, "backup.tar")
	if err := packageBackupFiles(dataDir, tarPath, NopLogWriter{}); err != nil {
		t.Fatalf("failed to create test tar: %v", err)
	}

	var capturedSQL string
	executor := &fakeCommandExecutor{
		runFunc: func(name string, args []string, options CommandOptions) error {
			capturedSQL = args[len(args)-1]
			return nil
		},
	}

	runner := NewSAPHANARunner(executor)
	err := runner.Restore(context.Background(), TaskSpec{
		Name: "hana-tenant-restore",
		Type: "saphana",
		Database: DatabaseSpec{
			Host: "10.0.0.1", Port: 30015, User: "SYSTEM", Password: "secret",
			Names: []string{"HDB"},
		},
	}, tarPath, NopLogWriter{})

	if err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}

	if !strings.Contains(capturedSQL, "RECOVER DATA FOR HDB USING FILE") {
		t.Fatalf("expected RECOVER DATA FOR HDB in SQL, got: %s", capturedSQL)
	}
}

func TestBuildBackupSQL_FullSystemDB(t *testing.T) {
	sql := buildBackupSQL("SYSTEMDB", []string{"/tmp/p1"}, "data", "full")
	if sql != "BACKUP DATA USING FILE ('/tmp/p1')" {
		t.Fatalf("unexpected SQL: %s", sql)
	}
}

func TestBuildBackupSQL_IncrementalTenant(t *testing.T) {
	sql := buildBackupSQL("HDB", []string{"/tmp/p1"}, "data", "incremental")
	expected := "BACKUP DATA FOR HDB INCREMENTAL USING FILE ('/tmp/p1')"
	if sql != expected {
		t.Fatalf("expected %q, got %q", expected, sql)
	}
}

func TestBuildBackupSQL_DifferentialTenant(t *testing.T) {
	sql := buildBackupSQL("HDB", []string{"/tmp/p1"}, "data", "differential")
	expected := "BACKUP DATA FOR HDB DIFFERENTIAL USING FILE ('/tmp/p1')"
	if sql != expected {
		t.Fatalf("expected %q, got %q", expected, sql)
	}
}

func TestBuildBackupSQL_LogBackup(t *testing.T) {
	sql := buildBackupSQL("HDB", []string{"/tmp/log"}, "log", "full")
	expected := "BACKUP LOG FOR HDB USING FILE ('/tmp/log')"
	if sql != expected {
		t.Fatalf("expected %q, got %q", expected, sql)
	}
}

func TestBuildBackupSQL_ParallelChannels(t *testing.T) {
	sql := buildBackupSQL("SYSTEMDB", []string{"/tmp/c0/p", "/tmp/c1/p", "/tmp/c2/p"}, "data", "full")
	expected := "BACKUP DATA USING FILE ('/tmp/c0/p', '/tmp/c1/p', '/tmp/c2/p')"
	if sql != expected {
		t.Fatalf("expected %q, got %q", expected, sql)
	}
}

func TestNormalizeBackupLevel(t *testing.T) {
	cases := map[string]string{
		"":             "full",
		"FULL":         "full",
		"incremental":  "incremental",
		"DIFFERENTIAL": "differential",
		"unknown":      "full",
	}
	for in, want := range cases {
		if got := normalizeBackupLevel(in); got != want {
			t.Errorf("normalizeBackupLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeBackupType(t *testing.T) {
	cases := map[string]string{
		"":     "data",
		"DATA": "data",
		"log":  "log",
		"LOG":  "log",
	}
	for in, want := range cases {
		if got := normalizeBackupType(in); got != want {
			t.Errorf("normalizeBackupType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSAPHANARunnerRun_IncrementalBackup(t *testing.T) {
	var capturedSQL string
	executor := &fakeCommandExecutor{
		runFunc: func(name string, args []string, options CommandOptions) error {
			capturedSQL = args[len(args)-1]
			startIdx := strings.Index(capturedSQL, "('") + 2
			endIdx := strings.Index(capturedSQL, "')")
			if startIdx > 1 && endIdx > startIdx {
				prefix := capturedSQL[startIdx:endIdx]
				_ = os.MkdirAll(filepath.Dir(prefix), 0o755)
				_ = os.WriteFile(prefix+"_databackup_0_1", []byte("incremental data"), 0o644)
			}
			return nil
		},
	}

	runner := NewSAPHANARunner(executor)
	result, err := runner.Run(context.Background(), TaskSpec{
		Name: "hana-incremental",
		Type: "saphana",
		Database: DatabaseSpec{
			Host:        "10.0.0.1",
			Port:        30015,
			User:        "SYSTEM",
			Password:    "secret",
			Names:       []string{"HDB"},
			BackupLevel: "incremental",
		},
	}, NopLogWriter{})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	defer os.RemoveAll(result.TempDir)

	if !strings.Contains(capturedSQL, "INCREMENTAL USING FILE") {
		t.Fatalf("expected INCREMENTAL in SQL, got: %s", capturedSQL)
	}
}

func TestSAPHANARunnerRun_LogBackup(t *testing.T) {
	var capturedSQL string
	executor := &fakeCommandExecutor{
		runFunc: func(name string, args []string, options CommandOptions) error {
			capturedSQL = args[len(args)-1]
			startIdx := strings.Index(capturedSQL, "('") + 2
			endIdx := strings.Index(capturedSQL, "')")
			if startIdx > 1 && endIdx > startIdx {
				prefix := capturedSQL[startIdx:endIdx]
				_ = os.MkdirAll(filepath.Dir(prefix), 0o755)
				_ = os.WriteFile(prefix+"_logbackup_0_1", []byte("log data"), 0o644)
			}
			return nil
		},
	}

	runner := NewSAPHANARunner(executor)
	result, err := runner.Run(context.Background(), TaskSpec{
		Name: "hana-log",
		Type: "saphana",
		Database: DatabaseSpec{
			Host: "10.0.0.1", Port: 30015, User: "SYSTEM", Password: "secret",
			Names:      []string{"HDB"},
			BackupType: "log",
		},
	}, NopLogWriter{})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	defer os.RemoveAll(result.TempDir)

	if !strings.Contains(capturedSQL, "BACKUP LOG FOR HDB USING FILE") {
		t.Fatalf("expected log backup SQL, got: %s", capturedSQL)
	}
}

func TestSAPHANARunnerRun_ParallelChannels(t *testing.T) {
	var capturedSQL string
	executor := &fakeCommandExecutor{
		runFunc: func(name string, args []string, options CommandOptions) error {
			capturedSQL = args[len(args)-1]
			// 模拟为每个通道生成备份文件
			parts := strings.Split(capturedSQL, "',")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if idx := strings.Index(p, "'"); idx >= 0 {
					prefix := p[idx+1:]
					prefix = strings.TrimSuffix(prefix, "')")
					prefix = strings.TrimSuffix(prefix, "'")
					if prefix != "" {
						_ = os.MkdirAll(filepath.Dir(prefix), 0o755)
						_ = os.WriteFile(prefix+"_databackup_0_1", []byte("data"), 0o644)
					}
				}
			}
			return nil
		},
	}

	runner := NewSAPHANARunner(executor)
	result, err := runner.Run(context.Background(), TaskSpec{
		Name: "hana-parallel",
		Type: "saphana",
		Database: DatabaseSpec{
			Host: "10.0.0.1", Port: 30015, User: "SYSTEM", Password: "secret",
			Names:          []string{"SYSTEMDB"},
			BackupChannels: 3,
		},
	}, NopLogWriter{})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	defer os.RemoveAll(result.TempDir)

	// 应该包含 3 个路径
	if strings.Count(capturedSQL, "'") != 6 { // 3 路径 × 2 引号
		t.Fatalf("expected 3 channels (6 quotes), got SQL: %s", capturedSQL)
	}
	if !strings.Contains(capturedSQL, "chan_0") || !strings.Contains(capturedSQL, "chan_2") {
		t.Fatalf("expected channel directories in SQL, got: %s", capturedSQL)
	}
}

func TestSAPHANARunnerRun_RetryOnFailure(t *testing.T) {
	attempts := 0
	executor := &fakeCommandExecutor{
		runFunc: func(name string, args []string, options CommandOptions) error {
			attempts++
			if attempts < 2 {
				return errors.New("transient failure")
			}
			// 第二次成功，写入备份文件
			sql := args[len(args)-1]
			startIdx := strings.Index(sql, "('") + 2
			endIdx := strings.Index(sql, "')")
			if startIdx > 1 && endIdx > startIdx {
				prefix := sql[startIdx:endIdx]
				_ = os.MkdirAll(filepath.Dir(prefix), 0o755)
				_ = os.WriteFile(prefix+"_databackup_0_1", []byte("data"), 0o644)
			}
			return nil
		},
	}

	// 使用极短的重试周期版本（这里通过 fake context 机制无法快进时间，所以直接验证 attempts）
	// 设置 MaxRetries=2 以加快测试，不会真实等待 5s
	runner := NewSAPHANARunner(executor)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := runner.Run(ctx, TaskSpec{
		Name: "hana-retry",
		Type: "saphana",
		Database: DatabaseSpec{
			Host: "10.0.0.1", Port: 30015, User: "SYSTEM", Password: "secret",
			Names:      []string{"SYSTEMDB"},
			MaxRetries: 2,
		},
	}, NopLogWriter{})

	if err != nil {
		t.Fatalf("Run returned error after retry: %v", err)
	}
	defer os.RemoveAll(result.TempDir)

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestHanaInstanceNumber(t *testing.T) {
	tests := []struct {
		port     int
		expected string
	}{
		{30015, "0"},
		{30115, "1"},
		{30215, "2"},
		{31015, "10"},
		{25000, "00"},
		{40001, "00"},
	}
	for _, tc := range tests {
		got := hanaInstanceNumber(tc.port)
		if got != tc.expected {
			t.Errorf("hanaInstanceNumber(%d) = %s, want %s", tc.port, got, tc.expected)
		}
	}
}
