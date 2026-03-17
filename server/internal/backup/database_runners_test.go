package backup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

type fakeCommandExecutor struct {
	lastName  string
	lastArgs  []string
	env       []string
	lookupErr error
	runFunc   func(name string, args []string, options CommandOptions) error
}

func (f *fakeCommandExecutor) LookPath(string) (string, error) {
	if f.lookupErr != nil {
		return "", f.lookupErr
	}
	return "/usr/bin/fake", nil
}

func (f *fakeCommandExecutor) Run(_ context.Context, name string, args []string, options CommandOptions) error {
	f.lastName = name
	f.lastArgs = append([]string{}, args...)
	f.env = append([]string{}, options.Env...)
	if f.runFunc != nil {
		return f.runFunc(name, args, options)
	}
	return nil
}

func TestMySQLRunnerUsesExpectedCommands(t *testing.T) {
	executor := &fakeCommandExecutor{runFunc: func(name string, args []string, options CommandOptions) error {
		if options.Stdout != nil {
			_, _ = io.WriteString(options.Stdout, "mysql dump")
		}
		return nil
	}}
	runner := NewMySQLRunner(executor)
	result, err := runner.Run(context.Background(), TaskSpec{Name: "mysql", Type: "mysql", Database: DatabaseSpec{Host: "127.0.0.1", Port: 3306, User: "root", Password: "secret", Names: []string{"app, audit"}}}, NopLogWriter{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if executor.lastName != "mysqldump" {
		t.Fatalf("expected mysqldump, got %s", executor.lastName)
	}
	if len(executor.lastArgs) == 0 || executor.lastArgs[len(executor.lastArgs)-2] != "app" || executor.lastArgs[len(executor.lastArgs)-1] != "audit" {
		t.Fatalf("unexpected mysql args: %#v", executor.lastArgs)
	}
	if _, err := os.Stat(result.ArtifactPath); err != nil {
		t.Fatalf("artifact file missing: %v", err)
	}
}

func TestPostgreSQLRunnerRestoreUsesPsql(t *testing.T) {
	executor := &fakeCommandExecutor{}
	runner := NewPostgreSQLRunner(executor)
	artifact := filepathJoinTempFile(t, "restore.sql", "select 1;")
	if err := runner.Restore(context.Background(), TaskSpec{Name: "postgres", Type: "postgresql", Database: DatabaseSpec{Host: "127.0.0.1", Port: 5432, User: "postgres", Password: "secret"}}, artifact, NopLogWriter{}); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if executor.lastName != "psql" {
		t.Fatalf("expected psql, got %s", executor.lastName)
	}
}

func TestMySQLRunnerReturnsLookupError(t *testing.T) {
	runner := NewMySQLRunner(&fakeCommandExecutor{lookupErr: errors.New("missing")})
	_, err := runner.Run(context.Background(), TaskSpec{Name: "mysql", Type: "mysql", Database: DatabaseSpec{Host: "127.0.0.1", Port: 3306, User: "root", Password: "secret", Names: []string{"app"}}}, NopLogWriter{})
	if err == nil {
		t.Fatal("expected error when mysqldump is missing")
	}
}

func filepathJoinTempFile(t *testing.T, name string, content string) string {
	t.Helper()
	filePath := t.TempDir() + "/" + name
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return filePath
}

func TestPostgreSQLRunnerRunAppendsMultipleDatabaseDumps(t *testing.T) {
	executor := &fakeCommandExecutor{runFunc: func(name string, args []string, options CommandOptions) error {
		_, _ = io.Copy(options.Stdout, bytes.NewBufferString(args[len(args)-1]))
		return nil
	}}
	runner := NewPostgreSQLRunner(executor)
	result, err := runner.Run(context.Background(), TaskSpec{Name: "pg", Type: "postgresql", Database: DatabaseSpec{Host: "127.0.0.1", Port: 5432, User: "postgres", Password: "secret", Names: []string{"app", "audit"}}}, NopLogWriter{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	content, err := os.ReadFile(result.ArtifactPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !bytes.Contains(content, []byte("app")) || !bytes.Contains(content, []byte("audit")) {
		t.Fatalf("unexpected pg dump content: %s", string(content))
	}
}
