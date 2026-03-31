package rclone

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestProviderLocalDiskCRUD(t *testing.T) {
	factory := NewLocalDiskFactory()
	provider, err := factory.New(context.Background(), map[string]any{"basePath": t.TempDir()})
	if err != nil {
		t.Fatalf("Factory.New returned error: %v", err)
	}
	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection returned error: %v", err)
	}

	// Upload
	if err := provider.Upload(context.Background(), "daily/backup.txt", strings.NewReader("hello"), 5, nil); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}

	// Download
	reader, err := provider.Download(context.Background(), "daily/backup.txt")
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != "hello" {
		t.Fatalf("expected 'hello', got %q", string(content))
	}

	// List with prefix
	items, err := provider.List(context.Background(), "daily")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].Key != "daily/backup.txt" {
		t.Fatalf("unexpected list result: %#v", items)
	}

	// Delete
	if err := provider.Delete(context.Background(), "daily/backup.txt"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	// List after delete should be empty
	items, err = provider.List(context.Background(), "daily")
	if err != nil {
		t.Fatalf("List after delete returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list after delete, got %d items", len(items))
	}
}

func TestProviderLocalDiskRequiresBasePath(t *testing.T) {
	_, err := NewLocalDiskFactory().New(context.Background(), map[string]any{"basePath": ""})
	if err == nil {
		t.Fatal("expected error for empty basePath")
	}
}

func TestProviderS3RequiresBucketAndCredentials(t *testing.T) {
	factory := NewS3Factory()
	_, err := factory.New(context.Background(), map[string]any{"bucket": "", "accessKeyId": "a", "secretAccessKey": "b"})
	if err == nil || !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("expected bucket required error, got %v", err)
	}
	_, err = factory.New(context.Background(), map[string]any{"bucket": "demo", "accessKeyId": "", "secretAccessKey": "b"})
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("expected credentials required error, got %v", err)
	}
}

func TestQuoteParam(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"", ""},
		{"has,comma", "'has,comma'"},
		{"has:colon", "'has:colon'"},
		{"has=equals", "'has=equals'"},
		{"has'quote", "'has''quote'"},
		{"a,b:c=d'e", "'a,b:c=d''e'"},
	}
	for _, tt := range tests {
		got := quoteParam(tt.input)
		if got != tt.expected {
			t.Errorf("quoteParam(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestBuildS3Remote(t *testing.T) {
	remote := buildS3Remote("Alibaba", "keyID", "secret", "https://oss-cn-hangzhou.aliyuncs.com", "cn-hangzhou", "my-bucket", false)
	if !strings.Contains(remote, "provider=Alibaba") {
		t.Fatalf("expected provider=Alibaba in remote: %s", remote)
	}
	if !strings.Contains(remote, ":my-bucket") {
		t.Fatalf("expected :my-bucket suffix in remote: %s", remote)
	}
	if !strings.HasPrefix(remote, ":s3,") {
		t.Fatalf("expected :s3, prefix in remote: %s", remote)
	}
}

func TestRcloneFactoryCRUD(t *testing.T) {
	dir := t.TempDir()
	factory := NewRcloneFactory()
	// 使用 rclone 的 local 后端
	provider, err := factory.New(context.Background(), map[string]any{
		"backend": "local",
		"root":    dir,
	})
	if err != nil {
		t.Fatalf("RcloneFactory.New returned error: %v", err)
	}
	if err := provider.Upload(context.Background(), "test.txt", strings.NewReader("rclone"), 6, nil); err != nil {
		t.Fatalf("Upload via rclone factory returned error: %v", err)
	}
	reader, err := provider.Download(context.Background(), "test.txt")
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != "rclone" {
		t.Fatalf("expected 'rclone', got %q", string(content))
	}
	if err := provider.Delete(context.Background(), "test.txt"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestRcloneFactoryRequiresBackend(t *testing.T) {
	_, err := NewRcloneFactory().New(context.Background(), map[string]any{"root": "/tmp"})
	if err == nil || !strings.Contains(err.Error(), "backend") {
		t.Fatalf("expected backend required error, got %v", err)
	}
}

func TestListBackends(t *testing.T) {
	backends := ListBackends()
	if len(backends) < 30 {
		t.Fatalf("expected at least 30 backends, got %d", len(backends))
	}
	// 确认 sftp 在列表中
	found := false
	for _, b := range backends {
		if b.Name == "sftp" {
			found = true
			if len(b.Options) == 0 {
				t.Fatal("sftp backend should have options")
			}
			break
		}
	}
	if !found {
		t.Fatal("sftp backend not found in ListBackends()")
	}
}

func TestProviderAbout(t *testing.T) {
	factory := NewLocalDiskFactory()
	provider, err := factory.New(context.Background(), map[string]any{"basePath": t.TempDir()})
	if err != nil {
		t.Fatalf("Factory.New returned error: %v", err)
	}
	// local 后端支持 About
	rcloneProvider := provider.(*Provider)
	usage, err := rcloneProvider.About(context.Background())
	if err != nil {
		t.Fatalf("About returned error: %v", err)
	}
	if usage.Total == nil || *usage.Total <= 0 {
		t.Fatalf("expected non-zero total disk space, got %v", usage.Total)
	}
}

func TestPathDir(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BackupX/file/260308/backup.tar.gz", "BackupX/file/260308"},
		{"backup.tar.gz", ""},
		{"a/b", "a"},
		{"", ""},
	}
	for _, tt := range tests {
		got := pathDir(tt.input)
		if got != tt.expected {
			t.Errorf("pathDir(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
