package localdisk

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"backupx/server/internal/storage"
)

type Provider struct {
	basePath string
}

type Factory struct{}

func NewFactory() Factory { return Factory{} }

func (Factory) Type() storage.ProviderType { return storage.ProviderTypeLocalDisk }
func (Factory) SensitiveFields() []string  { return nil }

func (Factory) New(_ context.Context, rawConfig map[string]any) (storage.StorageProvider, error) {
	cfg, err := storage.DecodeConfig[storage.LocalDiskConfig](rawConfig)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.BasePath) == "" {
		return nil, fmt.Errorf("local disk basePath is required")
	}
	return &Provider{basePath: filepath.Clean(cfg.BasePath)}, nil
}

func (p *Provider) Type() storage.ProviderType { return storage.ProviderTypeLocalDisk }

func (p *Provider) TestConnection(_ context.Context) error {
	if err := os.MkdirAll(p.basePath, 0o755); err != nil {
		return fmt.Errorf("ensure local disk base path: %w", err)
	}
	tempFile, err := os.CreateTemp(p.basePath, ".backupx-connection-test-*")
	if err != nil {
		return fmt.Errorf("write access check failed: %w", err)
	}
	name := tempFile.Name()
	_ = tempFile.Close()
	_ = os.Remove(name)
	return nil
}

func (p *Provider) Upload(_ context.Context, objectKey string, reader io.Reader, _ int64, _ map[string]string) error {
	targetPath, err := p.resolvePath(objectKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create local disk directories: %w", err)
	}
	file, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create local disk object: %w", err)
	}
	defer file.Close()
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("write local disk object: %w", err)
	}
	return nil
}

func (p *Provider) Download(_ context.Context, objectKey string) (io.ReadCloser, error) {
	targetPath, err := p.resolvePath(objectKey)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(targetPath)
	if err != nil {
		return nil, fmt.Errorf("open local disk object: %w", err)
	}
	return file, nil
}

func (p *Provider) Delete(_ context.Context, objectKey string) error {
	targetPath, err := p.resolvePath(objectKey)
	if err != nil {
		return err
	}
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete local disk object: %w", err)
	}
	return nil
}

func (p *Provider) List(_ context.Context, prefix string) ([]storage.ObjectInfo, error) {
	items := make([]storage.ObjectInfo, 0)
	err := filepath.WalkDir(p.basePath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(p.basePath, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		items = append(items, storage.ObjectInfo{Key: key, Size: info.Size(), UpdatedAt: info.ModTime().UTC()})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("list local disk objects: %w", err)
	}
	return items, nil
}

func (p *Provider) resolvePath(objectKey string) (string, error) {
	cleanBase := filepath.Clean(p.basePath)
	cleanKey := filepath.Clean(filepath.FromSlash(strings.TrimSpace(objectKey)))
	if cleanKey == "." || cleanKey == string(filepath.Separator) || cleanKey == "" {
		return "", fmt.Errorf("object key is required")
	}
	fullPath := filepath.Clean(filepath.Join(cleanBase, cleanKey))
	baseWithSep := cleanBase + string(filepath.Separator)
	if fullPath != cleanBase && !strings.HasPrefix(fullPath, baseWithSep) {
		return "", fmt.Errorf("object key escapes base path")
	}
	return fullPath, nil
}
