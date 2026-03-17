package webdav

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"backupx/server/internal/storage"
	gowebdav "github.com/studio-b12/gowebdav"
)

type client interface {
	ReadDir(path string) ([]os.FileInfo, error)
	WriteStream(path string, stream io.Reader, perm os.FileMode) error
	ReadStream(path string) (io.ReadCloser, error)
	Remove(path string) error
	MkdirAll(path string, perm os.FileMode) error
	Stat(path string) (os.FileInfo, error)
}

type Provider struct {
	client   client
	basePath string
}

type Factory struct {
	newClient func(cfg storage.WebDAVConfig) client
}

func NewFactory() Factory {
	return Factory{newClient: func(cfg storage.WebDAVConfig) client {
		return gowebdav.NewClient(strings.TrimRight(cfg.Endpoint, "/"), cfg.Username, cfg.Password)
	}}
}

func (Factory) Type() storage.ProviderType { return storage.ProviderTypeWebDAV }
func (Factory) SensitiveFields() []string  { return []string{"username", "password"} }

func (f Factory) New(_ context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[storage.WebDAVConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("webdav endpoint is required")
	}
	newClient := f.newClient
	if newClient == nil {
		factory := NewFactory()
		newClient = factory.newClient
	}
	return &Provider{client: newClient(cfg), basePath: normalizeBasePath(cfg.BasePath)}, nil
}

func (p *Provider) Type() storage.ProviderType { return storage.ProviderTypeWebDAV }

func (p *Provider) TestConnection(_ context.Context) error {
	if err := p.client.MkdirAll(p.basePath, 0o755); err != nil {
		return fmt.Errorf("ensure webdav base path: %w", err)
	}
	if _, err := p.client.Stat(p.basePath); err != nil {
		return fmt.Errorf("stat webdav base path: %w", err)
	}
	return nil
}

func (p *Provider) Upload(_ context.Context, objectKey string, reader io.Reader, _ int64, _ map[string]string) error {
	objectPath := p.resolvePath(objectKey)
	if err := p.client.MkdirAll(path.Dir(objectPath), 0o755); err != nil {
		return fmt.Errorf("create webdav directories: %w", err)
	}
	if err := p.client.WriteStream(objectPath, reader, 0o644); err != nil {
		return fmt.Errorf("write webdav object: %w", err)
	}
	return nil
}

func (p *Provider) Download(_ context.Context, objectKey string) (io.ReadCloser, error) {
	reader, err := p.client.ReadStream(p.resolvePath(objectKey))
	if err != nil {
		return nil, fmt.Errorf("read webdav object: %w", err)
	}
	return reader, nil
}

func (p *Provider) Delete(_ context.Context, objectKey string) error {
	if err := p.client.Remove(p.resolvePath(objectKey)); err != nil {
		return fmt.Errorf("delete webdav object: %w", err)
	}
	return nil
}

func (p *Provider) List(_ context.Context, prefix string) ([]storage.ObjectInfo, error) {
	entries, err := p.client.ReadDir(p.basePath)
	if err != nil {
		return nil, fmt.Errorf("list webdav directory: %w", err)
	}
	items := make([]storage.ObjectInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		key := strings.TrimPrefix(path.Join(strings.TrimPrefix(p.basePath, "/"), entry.Name()), "/")
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		items = append(items, storage.ObjectInfo{Key: key, Size: entry.Size(), UpdatedAt: entry.ModTime().UTC()})
	}
	return items, nil
}

func normalizeBasePath(value string) string {
	clean := path.Clean("/" + strings.TrimSpace(value))
	if clean == "." {
		return "/"
	}
	return clean
}

func (p *Provider) resolvePath(objectKey string) string {
	cleanKey := path.Clean("/" + strings.TrimSpace(objectKey))
	return path.Clean(path.Join(p.basePath, cleanKey))
}
