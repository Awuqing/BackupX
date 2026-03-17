package webdav

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"backupx/server/internal/storage"
)

type fakeFileInfo struct {
	name string
	size int64
	mod  time.Time
	dir  bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return f.mod }
func (f fakeFileInfo) IsDir() bool        { return f.dir }
func (f fakeFileInfo) Sys() any           { return nil }

type fakeClient struct{ data map[string]string }

func (c *fakeClient) ReadDir(_ string) ([]os.FileInfo, error) {
	return []os.FileInfo{fakeFileInfo{name: "backup.tar.gz", size: int64(len(c.data["/storage/backup.tar.gz"])), mod: time.Now().UTC()}}, nil
}
func (c *fakeClient) WriteStream(path string, stream io.Reader, _ os.FileMode) error {
	content, _ := io.ReadAll(stream)
	c.data[path] = string(content)
	return nil
}
func (c *fakeClient) ReadStream(path string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(c.data[path])), nil
}
func (c *fakeClient) Remove(path string) error               { delete(c.data, path); return nil }
func (c *fakeClient) MkdirAll(_ string, _ os.FileMode) error { return nil }
func (c *fakeClient) Stat(path string) (os.FileInfo, error) {
	return fakeFileInfo{name: path, dir: true}, nil
}

func TestWebDAVProviderCRUD(t *testing.T) {
	factory := Factory{newClient: func(storage.WebDAVConfig) client { return &fakeClient{data: make(map[string]string)} }}
	providerAny, err := factory.New(context.Background(), map[string]any{"endpoint": "http://dav.example.com", "basePath": "/storage"})
	if err != nil {
		t.Fatalf("Factory.New returned error: %v", err)
	}
	provider := providerAny.(*Provider)
	if err := provider.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection returned error: %v", err)
	}
	if err := provider.Upload(context.Background(), "backup.tar.gz", strings.NewReader("payload"), 7, nil); err != nil {
		t.Fatalf("Upload returned error: %v", err)
	}
	reader, err := provider.Download(context.Background(), "backup.tar.gz")
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != "payload" {
		t.Fatalf("unexpected content: %s", string(content))
	}
	items, err := provider.List(context.Background(), "storage")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].Key != "storage/backup.tar.gz" {
		t.Fatalf("unexpected list result: %#v", items)
	}
	if err := provider.Delete(context.Background(), "backup.tar.gz"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}
