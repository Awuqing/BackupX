package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteRunnerRunAndRestore(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "data.db")
	if err := os.WriteFile(dbPath, []byte("sqlite-data"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	runner := NewSQLiteRunner()
	result, err := runner.Run(context.Background(), TaskSpec{Name: "sqlite backup", Type: "sqlite", Database: DatabaseSpec{Path: dbPath}}, NopLogWriter{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := os.WriteFile(dbPath, []byte("mutated"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := runner.Restore(context.Background(), TaskSpec{Name: "sqlite backup", Type: "sqlite", Database: DatabaseSpec{Path: dbPath}}, result.ArtifactPath, NopLogWriter{}); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	content, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "sqlite-data" {
		t.Fatalf("unexpected restored content: %s", string(content))
	}
}
