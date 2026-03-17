package localdisk

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestLocalDiskProviderCRUD(t *testing.T) {
	providerAny, err := (Factory{}).New(context.Background(), map[string]any{"basePath": t.TempDir()})
	if err != nil {
		t.Fatalf("Factory.New returned error: %v", err)
	}
	provider := providerAny.(*Provider)
	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection returned error: %v", err)
	}
	if err := provider.Upload(context.Background(), "daily/backup.txt", strings.NewReader("hello"), 5, nil); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	reader, err := provider.Download(context.Background(), "daily/backup.txt")
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != "hello" {
		t.Fatalf("expected downloaded content to match, got %s", string(content))
	}
	items, err := provider.List(context.Background(), "daily")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].Key != "daily/backup.txt" {
		t.Fatalf("unexpected list result: %#v", items)
	}
	if err := provider.Delete(context.Background(), "daily/backup.txt"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

func TestLocalDiskProviderRejectsTraversal(t *testing.T) {
	providerAny, err := (Factory{}).New(context.Background(), map[string]any{"basePath": t.TempDir()})
	if err != nil {
		t.Fatalf("Factory.New returned error: %v", err)
	}
	provider := providerAny.(*Provider)
	if _, err := provider.resolvePath("../escape.txt"); err == nil {
		t.Fatalf("expected traversal to be rejected")
	}
}
