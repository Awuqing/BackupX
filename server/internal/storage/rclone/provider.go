package rclone

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"backupx/server/internal/storage"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/walk"
)

// Provider 包装 rclone fs.Fs，实现 storage.StorageProvider 接口。
type Provider struct {
	providerType storage.ProviderType
	rfs          fs.Fs
}

func newProvider(providerType storage.ProviderType, rfs fs.Fs) *Provider {
	return &Provider{providerType: providerType, rfs: rfs}
}

func (p *Provider) Type() storage.ProviderType { return p.providerType }

// TestConnection 通过列出根目录验证连通性。
func (p *Provider) TestConnection(ctx context.Context) error {
	_, err := p.rfs.List(ctx, "")
	if err != nil {
		return fmt.Errorf("rclone test connection: %w", err)
	}
	return nil
}

// Upload 通过 rclone fs.Fs.Put 上传文件。
func (p *Provider) Upload(ctx context.Context, objectKey string, reader io.Reader, size int64, _ map[string]string) error {
	dir := pathDir(objectKey)
	if dir != "" && dir != "." {
		if err := p.rfs.Mkdir(ctx, dir); err != nil {
			return fmt.Errorf("rclone mkdir %s: %w", dir, err)
		}
	}
	info := object.NewStaticObjectInfo(objectKey, time.Now(), size, true, nil, nil)
	if _, err := p.rfs.Put(ctx, reader, info); err != nil {
		return fmt.Errorf("rclone upload %s: %w", objectKey, err)
	}
	return nil
}

// Download 通过 rclone 获取对象并返回 io.ReadCloser。
func (p *Provider) Download(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	obj, err := p.rfs.NewObject(ctx, objectKey)
	if err != nil {
		return nil, fmt.Errorf("rclone find object %s: %w", objectKey, err)
	}
	reader, err := obj.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("rclone download %s: %w", objectKey, err)
	}
	return reader, nil
}

// Delete 通过 rclone 删除远端对象。
func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	obj, err := p.rfs.NewObject(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("rclone find object %s: %w", objectKey, err)
	}
	if err := obj.Remove(ctx); err != nil {
		return fmt.Errorf("rclone delete %s: %w", objectKey, err)
	}
	return nil
}

// List 递归列出指定前缀下的所有对象。
func (p *Provider) List(ctx context.Context, prefix string) ([]storage.ObjectInfo, error) {
	var items []storage.ObjectInfo
	err := walk.ListR(ctx, p.rfs, prefix, true, -1, walk.ListObjects, func(entries fs.DirEntries) error {
		for _, entry := range entries {
			obj, ok := entry.(fs.Object)
			if !ok {
				continue
			}
			key := obj.Remote()
			if prefix != "" && !strings.HasPrefix(key, prefix) {
				continue
			}
			items = append(items, storage.ObjectInfo{
				Key:       key,
				Size:      obj.Size(),
				UpdatedAt: obj.ModTime(ctx),
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("rclone list %s: %w", prefix, err)
	}
	return items, nil
}

// About 查询远端存储空间。并非所有 rclone 后端都支持。
func (p *Provider) About(ctx context.Context) (*storage.StorageUsageInfo, error) {
	about := p.rfs.Features().About
	if about == nil {
		return nil, fmt.Errorf("rclone about: backend %s does not support About", p.providerType)
	}
	usage, err := about(ctx)
	if err != nil {
		return nil, fmt.Errorf("rclone about: %w", err)
	}
	return &storage.StorageUsageInfo{
		Total:   usage.Total,
		Used:    usage.Used,
		Free:    usage.Free,
		Objects: usage.Objects,
	}, nil
}

// pathDir 返回 objectKey 的目录部分（正斜杠分隔）。
func pathDir(objectKey string) string {
	idx := strings.LastIndex(objectKey, "/")
	if idx < 0 {
		return ""
	}
	return objectKey[:idx]
}
