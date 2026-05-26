package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.bin")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// echo -n hello | sha256sum
	const want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	got, err := fileSHA256(p)
	if err != nil {
		t.Fatalf("fileSHA256: %v", err)
	}
	if got != want {
		t.Fatalf("fileSHA256 = %q, want %q", got, want)
	}
}

func TestVerifyArtifactChecksum(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "artifact.tar.gz")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sum = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	t.Run("empty expected skips (backward compat)", func(t *testing.T) {
		if err := verifyArtifactChecksum(p, ""); err != nil {
			t.Fatalf("empty expected should skip, got %v", err)
		}
	})
	t.Run("matching checksum passes (case-insensitive)", func(t *testing.T) {
		if err := verifyArtifactChecksum(p, strings.ToUpper(sum)); err != nil {
			t.Fatalf("matching checksum should pass, got %v", err)
		}
	})
	t.Run("mismatch is rejected", func(t *testing.T) {
		err := verifyArtifactChecksum(p, "deadbeef")
		if err == nil {
			t.Fatal("mismatch should error")
		}
		if !strings.Contains(err.Error(), "完整性校验失败") {
			t.Fatalf("unexpected error message: %v", err)
		}
	})
	t.Run("missing file errors", func(t *testing.T) {
		if err := verifyArtifactChecksum(filepath.Join(dir, "nope"), sum); err == nil {
			t.Fatal("missing file should error")
		}
	})
}
