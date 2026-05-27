package compress

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZstdRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "data.txt")
	content := []byte("hello zstd roundtrip 差异压缩测试 " + strings.Repeat("payload-", 2000))
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	compressed, err := ZstdFile(src)
	if err != nil {
		t.Fatalf("ZstdFile: %v", err)
	}
	if !strings.HasSuffix(compressed, ".zst") {
		t.Fatalf("expected .zst suffix, got %s", compressed)
	}
	// 删除原文件，确保后续读到的是解压结果而非残留原文件。
	if err := os.Remove(src); err != nil {
		t.Fatalf("remove source: %v", err)
	}
	out, err := UnzstdFile(compressed)
	if err != nil {
		t.Fatalf("UnzstdFile: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read decompressed: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("roundtrip mismatch: got %d bytes, want %d bytes", len(got), len(content))
	}
}
