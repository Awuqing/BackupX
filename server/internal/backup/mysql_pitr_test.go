package backup

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMySQLPITRFullArchiveAndPointInTimeRestore(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 14, 1, 0, 0, 0, time.UTC)
	incremental := false
	var decodedArgs []string
	var restored []string
	executor := &fakeCommandExecutor{runFunc: func(name string, args []string, options CommandOptions) error {
		switch name {
		case "mysql":
			if options.Stdin != nil {
				data, err := io.ReadAll(options.Stdin)
				restored = append(restored, string(data))
				return err
			}
			sql := args[len(args)-1]
			var text string
			switch sql {
			case "SELECT @@server_uuid, UTC_TIMESTAMP(), @@global.log_bin":
				text = "server-uuid\t2026-11-14 01:00:00\t1"
				if incremental {
					text = "server-uuid\t2026-11-14 02:00:00\t1"
				}
			case "SHOW BINARY LOG STATUS":
				text = "mysql-bin.000002\t64"
				if incremental {
					text = "mysql-bin.000003\t80"
				}
			case "SHOW BINARY LOGS":
				text = "mysql-bin.000001\t100\nmysql-bin.000002\t100\nmysql-bin.000003\t80"
			default:
				return errors.New("unexpected query: " + sql)
			}
			_, err := io.WriteString(options.Stdout, text)
			return err
		case "mysqldump":
			if !containsArg(args, "--source-data=2") || !containsArg(args, "--all-databases") || containsArg(args, "--password") {
				t.Fatalf("unsafe dump args %v", args)
			}
			_, err := io.WriteString(options.Stdout, "-- CHANGE REPLICATION SOURCE TO SOURCE_LOG_FILE='mysql-bin.000002', SOURCE_LOG_POS=32;\nCREATE DATABASE app;\n")
			return err
		case "mysqlbinlog":
			if containsArg(args, "--raw") {
				var prefix string
				for _, arg := range args {
					if strings.HasPrefix(arg, "--result-file=") {
						prefix = strings.TrimPrefix(arg, "--result-file=")
					}
				}
				return os.WriteFile(filepath.Join(prefix, args[len(args)-1]), make([]byte, 100), 0600)
			}
			decodedArgs = append([]string{}, args...)
			if !reflect.DeepEqual(options.Env, []string{"TZ=UTC"}) {
				t.Fatalf("binlog timezone: %v", options.Env)
			}
			_, err := io.WriteString(options.Stdout, "INSERT INTO app.events VALUES (1);")
			return err
		}
		return errors.New("unexpected command")
	}}
	runner := NewMySQLRunner(executor)
	task := TaskSpec{Name: "pitr", Type: "mysql", StartedAt: now, MySQLPITR: true, TempDir: t.TempDir(), Database: DatabaseSpec{Host: "db", Port: 3306, User: "backup", Password: "secret", Names: []string{"*"}}}
	full, err := runner.Run(ctx, task, NopLogWriter{})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(full.TempDir)
	if !full.MySQLCheckpoint.Full || full.MySQLCheckpoint.StartPosition != 32 {
		t.Fatalf("full checkpoint: %+v", full.MySQLCheckpoint)
	}
	incremental = true
	task.StartedAt = now.Add(time.Hour)
	task.MySQLBase = full.MySQLCheckpoint
	logs, err := runner.Run(ctx, task, NopLogWriter{})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(logs.TempDir)
	if !reflect.DeepEqual(logs.MySQLCheckpoint.Files, []string{"mysql-bin.000002", "mysql-bin.000003"}) {
		t.Fatalf("archive files: %v", logs.MySQLCheckpoint.Files)
	}
	task.RestoreTime = now.Add(30 * time.Minute)
	if err := runner.Restore(ctx, task, full.ArtifactPath, NopLogWriter{}); err != nil {
		t.Fatal(err)
	}
	if err := runner.Restore(ctx, task, logs.ArtifactPath, NopLogWriter{}); err != nil {
		t.Fatal(err)
	}
	if len(restored) != 2 || !strings.Contains(restored[0], "CREATE DATABASE") || !strings.Contains(restored[1], "INSERT INTO") {
		t.Fatalf("restore order: %v", restored)
	}
	for _, arg := range []string{"--start-position=32", "--stop-position=80", "--stop-datetime=2026-11-14 01:30:00", "--verify-binlog-checksum"} {
		if !containsArg(decodedArgs, arg) {
			t.Fatalf("missing %s in %v", arg, decodedArgs)
		}
	}
	task.RestoreTime = now.Add(time.Hour)
	if err := runner.Restore(ctx, task, logs.ArtifactPath, NopLogWriter{}); err == nil {
		t.Fatal("accepted end-exclusive boundary")
	}
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}

func TestMySQLCheckpointRejectsGapsAndPaths(t *testing.T) {
	now := time.Now().UTC()
	good := MySQLCheckpoint{ServerUUID: "uuid", Source: "db:3306", StartFile: "bin.000001", StartPosition: 4, EndFile: "bin.000002", EndPosition: 80, BaselineAt: now, CapturedAt: now.Add(time.Hour), Files: []string{"bin.000001", "bin.000002"}}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, files := range [][]string{{"bin.000001", "bin.000003", "bin.000002"}, {"../bin.000001", "bin.000002"}, {"bin.000001"}, {"bin.000001", "other.000002"}} {
		broken := good
		broken.Files = files
		if err := broken.Validate(); err == nil {
			t.Fatalf("accepted %v", files)
		}
	}
}

func TestMySQLPITRRejectsExpiredLogsAndChangedSource(t *testing.T) {
	now := time.Date(2026, 11, 14, 1, 0, 0, 0, time.UTC)
	for _, mode := range []string{"expired", "changed", "short"} {
		t.Run(mode, func(t *testing.T) {
			executor := &fakeCommandExecutor{runFunc: func(name string, args []string, options CommandOptions) error {
				if name == "mysqlbinlog" {
					var prefix string
					for _, arg := range args {
						if strings.HasPrefix(arg, "--result-file=") {
							prefix = strings.TrimPrefix(arg, "--result-file=")
						}
					}
					return os.WriteFile(filepath.Join(prefix, args[len(args)-1]), []byte("short"), 0600)
				}
				query := args[len(args)-1]
				text := ""
				switch query {
				case "SELECT @@server_uuid, UTC_TIMESTAMP(), @@global.log_bin":
					text = "uuid\t2026-11-14 02:00:00\t1"
					if mode == "changed" {
						text = "another\t2026-11-14 02:00:00\t1"
					}
				case "SHOW BINARY LOG STATUS":
					text = "bin.000002\t80"
				case "SHOW BINARY LOGS":
					text = "bin.000001\t80\nbin.000002\t80"
					if mode == "expired" {
						text = "bin.000002\t80"
					}
				default:
					return errors.New("unexpected query")
				}
				_, err := io.WriteString(options.Stdout, text)
				return err
			}}
			base := &MySQLCheckpoint{ServerUUID: "uuid", Source: "db:3306", StartFile: "bin.000001", StartPosition: 4, EndFile: "bin.000001", EndPosition: 80, BaselineAt: now, CapturedAt: now, Full: true}
			_, err := NewMySQLRunner(executor).Run(context.Background(), TaskSpec{Name: "reject-" + mode, StartedAt: now, MySQLPITR: true, MySQLBase: base, Database: DatabaseSpec{Host: "db", Port: 3306, Names: []string{"*"}}}, NopLogWriter{})
			if err == nil {
				t.Fatal("accepted " + mode)
			}
		})
	}
}
