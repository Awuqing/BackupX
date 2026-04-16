package backint

import (
	"path/filepath"
	"testing"
)

func TestCatalog_CRUD(t *testing.T) {
	dir := t.TempDir()
	cat, err := OpenCatalog(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer cat.Close()

	if err := cat.Put(CatalogEntry{EBID: "bid-1", ObjectKey: "k/1.bin", SourcePath: "/tmp/a", Size: 100}); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := cat.Put(CatalogEntry{EBID: "bid-2", ObjectKey: "k/2.bin", Size: 200}); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := cat.Get("bid-1")
	if err != nil || got == nil {
		t.Fatalf("get: %v %v", got, err)
	}
	if got.ObjectKey != "k/1.bin" || got.Size != 100 {
		t.Errorf("mismatch: %+v", got)
	}

	// 不存在的条目
	missing, err := cat.Get("bid-999")
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil, got %+v", missing)
	}

	// List
	all, err := cat.List()
	if err != nil || len(all) != 2 {
		t.Fatalf("list: %v %d", err, len(all))
	}

	// Delete
	if err := cat.Delete("bid-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	got, _ = cat.Get("bid-1")
	if got != nil {
		t.Errorf("bid-1 should be deleted")
	}
}

func TestCatalog_UpsertSameEBID(t *testing.T) {
	dir := t.TempDir()
	cat, err := OpenCatalog(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer cat.Close()

	if err := cat.Put(CatalogEntry{EBID: "bid-x", ObjectKey: "v1"}); err != nil {
		t.Fatal(err)
	}
	if err := cat.Put(CatalogEntry{EBID: "bid-x", ObjectKey: "v2"}); err != nil {
		t.Fatal(err)
	}
	got, _ := cat.Get("bid-x")
	if got == nil || got.ObjectKey != "v2" {
		t.Errorf("upsert failed: %+v", got)
	}
}
