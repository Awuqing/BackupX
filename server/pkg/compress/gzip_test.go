package compress

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGzipAndGunzipFile(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(sourcePath, []byte("payload"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	compressedPath, err := GzipFile(sourcePath)
	if err != nil {
		t.Fatalf("GzipFile returned error: %v", err)
	}
	decompressedPath, err := GunzipFile(compressedPath)
	if err != nil {
		t.Fatalf("GunzipFile returned error: %v", err)
	}
	content, err := os.ReadFile(decompressedPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "payload" {
		t.Fatalf("unexpected decompressed content: %s", string(content))
	}
}
