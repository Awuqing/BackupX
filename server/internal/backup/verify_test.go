package backup

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// 构造一个最小的 tar 归档文件供测试使用
func writeTestTar(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.tar")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	for name, body := range entries {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	_ = tw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write tar file: %v", err)
	}
	return path
}

func TestVerifyTarArchive_Valid(t *testing.T) {
	path := writeTestTar(t, map[string][]byte{
		"readme.md":  []byte("hello"),
		"data.bin":   []byte("world!!!"),
	})
	report, err := VerifyTarArchive(path, "")
	if err != nil {
		t.Fatalf("VerifyTarArchive returned error: %v", err)
	}
	if report.TotalEntries != 2 {
		t.Fatalf("expected 2 entries, got %d", report.TotalEntries)
	}
	if report.FileBytes == 0 {
		t.Fatalf("expected non-zero file bytes")
	}
	if !report.ChecksumOK {
		t.Fatalf("checksumOK should be true when expected checksum empty")
	}
}

func TestVerifyTarArchive_Truncated(t *testing.T) {
	// 构造带多个大 entry 的 tar，在 entry 数据中间截断，使 io.Copy 触发 UnexpectedEOF
	path := filepath.Join(t.TempDir(), "big.tar")
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	body := bytes.Repeat([]byte("x"), 4096)
	_ = tw.WriteHeader(&tar.Header{Name: "big.bin", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
	_, _ = tw.Write(body)
	_ = tw.Close()
	data := buf.Bytes()
	// 保留 header 完整（512），破坏 body 中间使 tar.Reader 在 io.Copy 时遇到 EOF
	truncated := data[:512+1024]
	if err := os.WriteFile(path, truncated, 0o644); err != nil {
		t.Fatalf("write truncated: %v", err)
	}
	if _, err := VerifyTarArchive(path, ""); err == nil {
		t.Fatalf("expected error on truncated tar, got nil")
	}
}

func TestVerifySQLiteFile_Valid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.db")
	content := []byte("SQLite format 3\x00" + string(make([]byte, 100)))
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	report, err := VerifySQLiteFile(path)
	if err != nil {
		t.Fatalf("VerifySQLiteFile: %v", err)
	}
	if report.FileBytes == 0 {
		t.Fatalf("expected non-zero size")
	}
}

func TestVerifySQLiteFile_Invalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.db")
	if err := os.WriteFile(path, []byte("not sqlite at all, some other text"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := VerifySQLiteFile(path); err == nil {
		t.Fatalf("expected error on non-sqlite file")
	}
}

func TestVerifyMySQLDump(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dump.sql")
	content := "-- MySQL dump 10.13  Distrib 8.0.33\n-- Host: localhost\nINSERT INTO foo VALUES (1);\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	report, err := VerifyMySQLDump(path)
	if err != nil {
		t.Fatalf("VerifyMySQLDump: %v", err)
	}
	if report.Detail == "" {
		t.Fatalf("expected Detail in report")
	}
}

func TestVerifyPostgreSQLDump_Invalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notpg.sql")
	if err := os.WriteFile(path, []byte("some random text without header markers"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := VerifyPostgreSQLDump(path); err == nil {
		t.Fatalf("expected error on non-pg dump")
	}
}
