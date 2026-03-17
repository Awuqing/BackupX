package storage

import (
	"context"
	"io"
	"strings"
	"testing"
)

type fakeProvider struct{}

func (fakeProvider) Type() ProviderType                   { return ProviderTypeLocalDisk }
func (fakeProvider) TestConnection(context.Context) error { return nil }
func (fakeProvider) Upload(context.Context, string, io.Reader, int64, map[string]string) error {
	return nil
}
func (fakeProvider) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("ok")), nil
}
func (fakeProvider) Delete(context.Context, string) error               { return nil }
func (fakeProvider) List(context.Context, string) ([]ObjectInfo, error) { return nil, nil }

type fakeFactory struct{}

func (fakeFactory) Type() ProviderType        { return ProviderTypeLocalDisk }
func (fakeFactory) SensitiveFields() []string { return []string{"secret"} }
func (fakeFactory) New(context.Context, map[string]any) (StorageProvider, error) {
	return fakeProvider{}, nil
}

func TestRegistryCreate(t *testing.T) {
	registry := NewRegistry(fakeFactory{})
	provider, err := registry.Create(context.Background(), ProviderTypeLocalDisk, map[string]any{"basePath": "/tmp"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if provider.Type() != ProviderTypeLocalDisk {
		t.Fatalf("expected local disk provider, got %s", provider.Type())
	}
}

func TestRegistryCreateReturnsErrorForUnknownType(t *testing.T) {
	registry := NewRegistry()
	_, err := registry.Create(context.Background(), ProviderTypeS3, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported type error, got %v", err)
	}
}

func TestDecodeConfig(t *testing.T) {
	cfg, err := DecodeConfig[LocalDiskConfig](map[string]any{"basePath": "/tmp/storage"})
	if err != nil {
		t.Fatalf("DecodeConfig returned error: %v", err)
	}
	if cfg.BasePath != "/tmp/storage" {
		t.Fatalf("expected base path to decode")
	}
}
