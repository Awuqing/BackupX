package backup

import (
	"archive/tar"
	"context"
	"os"
	"path/filepath"
	"testing"
)

type bufferWriter struct{ lines []string }

func (w *bufferWriter) WriteLine(message string) { w.lines = append(w.lines, message) }

func TestFileRunnerRunAndRestore(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "site")
	if err := os.MkdirAll(filepath.Join(sourceDir, "node_modules"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "app.log"), []byte("skip"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "node_modules", "pkg.json"), []byte("skip-dir"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	runner := NewFileRunner()
	writer := &bufferWriter{}
	result, err := runner.Run(context.Background(), TaskSpec{Name: "site files", Type: "file", SourcePath: sourceDir, ExcludePatterns: []string{"*.log", "node_modules"}}, writer)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	archiveFile, err := os.Open(result.ArtifactPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer archiveFile.Close()
	reader := tar.NewReader(archiveFile)
	entries := map[string]bool{}
	for {
		header, err := reader.Next()
		if err != nil {
			break
		}
		entries[header.Name] = true
	}
	if !entries["site/index.html"] {
		t.Fatalf("expected site/index.html in archive, got %#v", entries)
	}
	if entries["site/app.log"] || entries["site/node_modules/pkg.json"] {
		t.Fatalf("unexpected excluded entries: %#v", entries)
	}
	if err := os.RemoveAll(sourceDir); err != nil {
		t.Fatalf("RemoveAll returned error: %v", err)
	}
	if err := runner.Restore(context.Background(), TaskSpec{Name: "site files", Type: "file", SourcePath: sourceDir}, result.ArtifactPath, writer); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(sourceDir, "index.html"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "ok" {
		t.Fatalf("unexpected restored content: %s", string(content))
	}
}
