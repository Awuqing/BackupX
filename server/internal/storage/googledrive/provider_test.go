package googledrive

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"backupx/server/internal/storage"
)

type fakeClient struct{ data map[string]string }

func (c *fakeClient) TestConnection(context.Context, string) error { return nil }
func (c *fakeClient) Upload(_ context.Context, _ string, objectKey string, reader io.Reader) error {
	content, _ := io.ReadAll(reader)
	c.data[objectKey] = string(content)
	return nil
}
func (c *fakeClient) Download(_ context.Context, _ string, objectKey string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(c.data[objectKey])), nil
}
func (c *fakeClient) Delete(_ context.Context, _ string, objectKey string) error {
	delete(c.data, objectKey)
	return nil
}
func (c *fakeClient) List(_ context.Context, _ string, prefix string) ([]storage.ObjectInfo, error) {
	items := make([]storage.ObjectInfo, 0)
	for key, value := range c.data {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			items = append(items, storage.ObjectInfo{Key: key, Size: int64(len(value)), UpdatedAt: time.Now().UTC()})
		}
	}
	return items, nil
}
func (c *fakeClient) EnsureFolder(_ context.Context, _, name string) (string, error) {
	return "fake-folder-" + name, nil
}

func TestGoogleDriveProviderCRUD(t *testing.T) {
	factory := Factory{newClient: func(context.Context, storage.GoogleDriveConfig) (client, error) {
		return &fakeClient{data: make(map[string]string)}, nil
	}}
	providerAny, err := factory.New(context.Background(), map[string]any{"clientId": "id", "clientSecret": "secret", "refreshToken": "refresh"})
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
	items, err := provider.List(context.Background(), "backup")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].Key != "backup.tar.gz" {
		t.Fatalf("unexpected list result: %#v", items)
	}
	if err := provider.Delete(context.Background(), "backup.tar.gz"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}
