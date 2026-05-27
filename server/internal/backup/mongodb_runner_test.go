package backup

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func argIndex(args []string, target string) int {
	for i, a := range args {
		if a == target {
			return i
		}
	}
	return -1
}

func TestMongoDBRunnerRunUsesMongodump(t *testing.T) {
	executor := &fakeCommandExecutor{runFunc: func(name string, args []string, options CommandOptions) error {
		if options.Stdout != nil {
			_, _ = io.WriteString(options.Stdout, "mongo archive bytes")
		}
		return nil
	}}
	runner := NewMongoDBRunner(executor)
	result, err := runner.Run(context.Background(), TaskSpec{Name: "mongo", Type: "mongodb", Database: DatabaseSpec{Host: "127.0.0.1", Port: 27017, User: "admin", Password: "secret", Names: []string{"app"}}}, NopLogWriter{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if executor.lastName != "mongodump" {
		t.Fatalf("expected mongodump, got %s", executor.lastName)
	}
	args := executor.lastArgs
	if argIndex(args, "--archive") < 0 {
		t.Fatalf("expected --archive flag, got %#v", args)
	}
	if i := argIndex(args, "--db"); i < 0 || i+1 >= len(args) || args[i+1] != "app" {
		t.Fatalf("expected --db app, got %#v", args)
	}
	if i := argIndex(args, "--username"); i < 0 || args[i+1] != "admin" {
		t.Fatalf("expected --username admin, got %#v", args)
	}
	if argIndex(args, "--authenticationDatabase") < 0 || argIndex(args, "--password") < 0 {
		t.Fatalf("expected auth args, got %#v", args)
	}
	if _, err := os.Stat(result.ArtifactPath); err != nil {
		t.Fatalf("artifact file missing: %v", err)
	}
	if result.StorageKey == "" || !strings.HasSuffix(result.FileName, ".archive") {
		t.Fatalf("unexpected result metadata: %#v", result)
	}
}

func TestMongoDBRunnerRunBackupsAllWhenNoDatabase(t *testing.T) {
	executor := &fakeCommandExecutor{runFunc: func(name string, args []string, options CommandOptions) error {
		_, _ = io.WriteString(options.Stdout, "all dbs")
		return nil
	}}
	runner := NewMongoDBRunner(executor)
	_, err := runner.Run(context.Background(), TaskSpec{Name: "mongo", Type: "mongodb", Database: DatabaseSpec{Host: "127.0.0.1", Port: 27017}}, NopLogWriter{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if argIndex(executor.lastArgs, "--db") >= 0 {
		t.Fatalf("expected no --db when backing up all databases, got %#v", executor.lastArgs)
	}
}

func TestMongoDBRunnerRunRejectsEmptyOutput(t *testing.T) {
	executor := &fakeCommandExecutor{} // runFunc nil → writes nothing
	runner := NewMongoDBRunner(executor)
	_, err := runner.Run(context.Background(), TaskSpec{Name: "mongo", Type: "mongodb", Database: DatabaseSpec{Host: "127.0.0.1", Port: 27017, Names: []string{"app"}}}, NopLogWriter{})
	if err == nil {
		t.Fatal("expected error for empty mongodump output")
	}
}

func TestMongoDBRunnerRestoreUsesMongorestore(t *testing.T) {
	executor := &fakeCommandExecutor{}
	runner := NewMongoDBRunner(executor)
	artifact := filepathJoinTempFile(t, "dump.archive", "mongo archive bytes")
	if err := runner.Restore(context.Background(), TaskSpec{Name: "mongo", Type: "mongodb", Database: DatabaseSpec{Host: "127.0.0.1", Port: 27017, User: "admin", Password: "secret"}}, artifact, NopLogWriter{}); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if executor.lastName != "mongorestore" {
		t.Fatalf("expected mongorestore, got %s", executor.lastName)
	}
	if argIndex(executor.lastArgs, "--drop") < 0 || argIndex(executor.lastArgs, "--archive") < 0 {
		t.Fatalf("expected --drop --archive, got %#v", executor.lastArgs)
	}
}

func TestMongoDBRunnerRunReturnsLookupError(t *testing.T) {
	runner := NewMongoDBRunner(&fakeCommandExecutor{lookupErr: errors.New("missing")})
	_, err := runner.Run(context.Background(), TaskSpec{Name: "mongo", Type: "mongodb", Database: DatabaseSpec{Host: "127.0.0.1", Port: 27017, Names: []string{"app"}}}, NopLogWriter{})
	if err == nil {
		t.Fatal("expected error when mongodump is missing")
	}
}
