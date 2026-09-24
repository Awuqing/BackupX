package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Run the test executable as a standalone worker to exercise real pipes, process
// exit codes and cancellation on both operating systems, without a SQL instance.
func init() {
	scenario := os.Getenv("BACKUPX_TEST_SQLVDI_SCENARIO")
	if scenario == "" {
		return
	}
	if len(os.Args) != 5 {
		os.Exit(9)
	}
	if strings.Contains(strings.Join(os.Args, " "), "test-password") {
		os.Exit(8)
	}
	if scenario == "bad_ready" {
		if _, err := fmt.Fprintln(os.Stdout, "wrong protocol"); err != nil {
			os.Exit(7)
		}
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			os.Exit(7)
		}
		os.Exit(2)
	}
	path := os.Args[3]
	content := "VDI backup data"
	if scenario == "empty" {
		content = ""
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		os.Exit(3)
	}
	if _, err := fmt.Fprintln(os.Stdout, "BACKUPX_SQLVDI_READY"); err != nil {
		os.Exit(7)
	}
	if scenario == "wait_cancel" {
		if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
			os.Exit(4)
		}
		os.Exit(2)
	}
	deadline := time.NewTimer(3 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	for {
		select {
		case <-deadline.C:
			os.Exit(5)
		case <-ticker.C:
			if _, err := os.Stat(os.Getenv("BACKUPX_TEST_SQLVDI_MARKER")); err == nil {
				if scenario == "helper_failure" {
					os.Exit(6)
				}
				os.Exit(0)
			}
		}
	}
}

func sqlServerTestSpec(t *testing.T) TaskSpec {
	t.Helper()
	return TaskSpec{Name: "sqlserver", Type: "sqlserver", TempDir: t.TempDir(), Database: DatabaseSpec{
		Host: "localhost", Port: 1433, User: "backup", Password: "test-password", Names: []string{"app]db"},
		ExtraConfig: `{"instanceName":"TEST","trustServerCertificate":true}`,
	}}
}

func TestSQLServerRunnerCompletionAndFailure(t *testing.T) {
	for _, scenario := range []string{"success", "empty", "sql_failure", "helper_failure", "bad_ready", "cancel"} {
		t.Run(scenario, func(t *testing.T) {
			spec := sqlServerTestSpec(t)
			marker := filepath.Join(t.TempDir(), "sql-started")
			t.Setenv("BACKUPX_TEST_SQLVDI_MARKER", marker)
			mode := scenario
			if mode == "sql_failure" || mode == "cancel" {
				mode = "wait_cancel"
			}
			t.Setenv("BACKUPX_TEST_SQLVDI_SCENARIO", mode)
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			runner := &SQLServerRunner{helperPath: binary, execSQL: func(ctx context.Context, db DatabaseSpec, opts SQLServerOptions, query, device string) error {
				if !opts.TrustServerCertificate || opts.InstanceName != "TEST" || !strings.Contains(query, "[app]]db]") || !strings.HasPrefix(device, "BackupX-") {
					return errors.New("SQL options or escaped identifier missing")
				}
				if scenario == "sql_failure" {
					return errors.New("denied test-password")
				}
				if scenario == "cancel" {
					cancel()
					return ctx.Err()
				}
				if err := os.WriteFile(marker, nil, 0o600); err != nil {
					return err
				}
				if scenario == "helper_failure" {
					<-ctx.Done()
					return ctx.Err()
				}
				return nil
			}}
			result, err := runner.Run(ctx, spec, NopLogWriter{})
			if scenario == "success" {
				if err != nil || result == nil || result.Size == 0 || !strings.HasSuffix(result.FileName, ".bak") {
					t.Fatalf("expected artifact, got %+v, %v", result, err)
				}
				return
			}
			if err == nil || result != nil || strings.Contains(err.Error(), "test-password") {
				t.Fatalf("failure must not succeed or leak credentials: result=%+v err=%v", result, err)
			}
			entries, readErr := os.ReadDir(spec.TempDir)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("failed backup left temporary artifacts: %v %v", entries, readErr)
			}
		})
	}
}

func TestSQLServerValidationAndRestoreSQL(t *testing.T) {
	db := sqlServerTestSpec(t).Database
	for _, mutate := range []func(*DatabaseSpec){
		func(d *DatabaseSpec) { d.Host = "remote.example.com" },
		func(d *DatabaseSpec) { d.Names = []string{"one,two"} },
		func(d *DatabaseSpec) { d.Names = []string{"one", "two"} },
		func(d *DatabaseSpec) { d.Names = []string{"tempdb"} },
		func(d *DatabaseSpec) { d.Port = 65536 },
		func(d *DatabaseSpec) { d.ExtraConfig = `{"trustServerCertificate":"true"}` },
	} {
		invalid := db
		mutate(&invalid)
		if _, err := ValidateSQLServerDatabase(invalid); err == nil {
			t.Fatalf("accepted invalid database: %+v", invalid)
		}
	}
	query := sqlServerQuery("restore", "db]; DROP DATABASE [other")
	if query != "RESTORE DATABASE [db]]; DROP DATABASE [other] FROM VIRTUAL_DEVICE = @device WITH CHECKSUM" || strings.Contains(query, "REPLACE") {
		t.Fatalf("unsafe restore query: %s", query)
	}
	if !strings.Contains(sqlServerQuery("backup", "app"), "COPY_ONLY, CHECKSUM") {
		t.Fatal("backup must preserve the existing differential base")
	}
}

func TestSQLServerConnectionCancellationWithoutServerResponse(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := listener.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- executeSQLServer(ctx, DatabaseSpec{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, User: "test", Password: "secret"}, SQLServerOptions{}, "SELECT 1", "")
	}()
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	}()
	// Keep the socket open and send no TDS response: cancellation must still finish.
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled SQL connection succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SQL cancellation waited for an unresponsive server")
	}
}
