package backint

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"backupx/server/internal/storage"
	storageRclone "backupx/server/internal/storage/rclone"
)

// Agent 是 Backint 协议代理主入口。
//
// 职责：
//  1. 根据 -f 指定的功能，从 -i 输入文件解析请求
//  2. 把数据路由到 BackupX storage 后端
//  3. 把结果写回 -o 输出文件（失败使用 #ERROR，不中断批次）
type Agent struct {
	cfg      *Config
	provider storage.StorageProvider
	catalog  *Catalog
}

// NewAgent 构造 Agent，初始化 storage provider 与 catalog。
func NewAgent(ctx context.Context, cfg *Config) (*Agent, error) {
	registry := buildStorageRegistry()
	provider, err := registry.Create(ctx, cfg.StorageType, cfg.StorageConfig)
	if err != nil {
		return nil, fmt.Errorf("create storage provider: %w", err)
	}
	if err := provider.TestConnection(ctx); err != nil {
		return nil, fmt.Errorf("storage provider connection failed: %w", err)
	}
	cat, err := OpenCatalog(cfg.CatalogDB)
	if err != nil {
		return nil, err
	}
	return &Agent{cfg: cfg, provider: provider, catalog: cat}, nil
}

// Close 释放资源。
func (a *Agent) Close() error {
	if a.catalog != nil {
		return a.catalog.Close()
	}
	return nil
}

// Run 执行一次 Backint 调用。
//
// HANA 针对 BACKUP 调用时：input 是 #PIPE 列表，output 需返回 #SAVED 或 #ERROR。
// 批次中任一条目失败不应导致整个进程退出，因此错误被降级为 #ERROR 行。
// 仅在极端错误（参数非法、I/O 失败）时返回 error，进程以非 0 退出。
func (a *Agent) Run(ctx context.Context, fn Function, inputPath, outputPath string) error {
	in, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer in.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	switch fn {
	case FunctionBackup:
		return a.runBackup(ctx, in, out)
	case FunctionRestore:
		return a.runRestore(ctx, in, out)
	case FunctionInquire:
		return a.runInquire(ctx, in, out)
	case FunctionDelete:
		return a.runDelete(ctx, in, out)
	default:
		return fmt.Errorf("unsupported function: %s", fn)
	}
}

// runBackup 处理 BACKUP 操作：读取每条请求的管道/文件，上传到存储后端。
func (a *Agent) runBackup(ctx context.Context, in io.Reader, out io.Writer) error {
	reqs, err := ParseBackupRequests(in)
	if err != nil {
		return err
	}
	for _, req := range reqs {
		ebid, perr := a.handleBackupOne(ctx, req)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "backint: backup %q failed: %v\n", req.Path, perr)
			_ = WriteError(out, req.Path)
			continue
		}
		_ = WriteSaved(out, ebid, req.Path)
	}
	return nil
}

// handleBackupOne 上传一条请求，返回分配的 EBID。
func (a *Agent) handleBackupOne(ctx context.Context, req BackupRequest) (string, error) {
	src, size, err := openBackupSource(req)
	if err != nil {
		return "", err
	}
	defer src.Close()

	ebid := generateEBID()
	objectKey := a.objectKeyFor(ebid)

	reader := io.Reader(src)
	// 可选 gzip 压缩
	if a.cfg.Compress {
		pr, pw := io.Pipe()
		go func() {
			gw := gzip.NewWriter(pw)
			if _, cerr := io.Copy(gw, src); cerr != nil {
				_ = gw.Close()
				_ = pw.CloseWithError(cerr)
				return
			}
			if cerr := gw.Close(); cerr != nil {
				_ = pw.CloseWithError(cerr)
				return
			}
			_ = pw.Close()
		}()
		reader = pr
		size = -1 // 压缩后大小未知
		objectKey += ".gz"
	}

	meta := map[string]string{
		"source-path": req.Path,
		"ebid":        ebid,
		"compress":    boolStr(a.cfg.Compress),
	}
	if err := a.provider.Upload(ctx, objectKey, reader, size, meta); err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}

	if err := a.catalog.Put(CatalogEntry{
		EBID:       ebid,
		ObjectKey:  objectKey,
		SourcePath: req.Path,
		Size:       size,
	}); err != nil {
		return "", fmt.Errorf("catalog put: %w", err)
	}
	return ebid, nil
}

// runRestore 处理 RESTORE 操作：根据 EBID 从存储下载，写入 HANA 指定的管道/文件。
func (a *Agent) runRestore(ctx context.Context, in io.Reader, out io.Writer) error {
	reqs, err := ParseRestoreRequests(in)
	if err != nil {
		return err
	}
	for _, req := range reqs {
		if perr := a.handleRestoreOne(ctx, req); perr != nil {
			fmt.Fprintf(os.Stderr, "backint: restore %q failed: %v\n", req.EBID, perr)
			_ = WriteError(out, req.Path)
			continue
		}
		_ = WriteRestored(out, req.EBID, req.Path)
	}
	return nil
}

func (a *Agent) handleRestoreOne(ctx context.Context, req RestoreRequest) error {
	entry, err := a.catalog.Get(req.EBID)
	if err != nil {
		return fmt.Errorf("catalog get: %w", err)
	}
	if entry == nil {
		return fmt.Errorf("ebid not found: %s", req.EBID)
	}
	rc, err := a.provider.Download(ctx, entry.ObjectKey)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer rc.Close()

	var src io.Reader = rc
	if strings.HasSuffix(entry.ObjectKey, ".gz") {
		gr, err := gzip.NewReader(rc)
		if err != nil {
			return fmt.Errorf("gzip reader: %w", err)
		}
		defer gr.Close()
		src = gr
	}

	dst, err := openRestoreTarget(req)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy to target: %w", err)
	}
	return nil
}

// runInquire 处理 INQUIRE 操作：查询 EBID 是否存在，或列出全部备份。
func (a *Agent) runInquire(ctx context.Context, in io.Reader, out io.Writer) error {
	reqs, err := ParseInquireRequests(in)
	if err != nil {
		return err
	}
	for _, req := range reqs {
		if req.All {
			entries, err := a.catalog.List()
			if err != nil {
				fmt.Fprintf(os.Stderr, "backint: inquire list failed: %v\n", err)
				_ = WriteError(out, "#NULL")
				continue
			}
			for _, e := range entries {
				_ = WriteBackup(out, e.EBID)
			}
			continue
		}
		entry, err := a.catalog.Get(req.EBID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "backint: inquire %q failed: %v\n", req.EBID, err)
			_ = WriteError(out, req.EBID)
			continue
		}
		if entry == nil {
			_ = WriteNotFound(out, req.EBID)
			continue
		}
		_ = WriteBackup(out, entry.EBID)
	}
	return nil
}

// runDelete 处理 DELETE 操作：从存储删除对象并移除目录条目。
func (a *Agent) runDelete(ctx context.Context, in io.Reader, out io.Writer) error {
	reqs, err := ParseDeleteRequests(in)
	if err != nil {
		return err
	}
	for _, req := range reqs {
		if perr := a.handleDeleteOne(ctx, req); perr != nil {
			fmt.Fprintf(os.Stderr, "backint: delete %q failed: %v\n", req.EBID, perr)
			_ = WriteError(out, req.EBID)
			continue
		}
		_ = WriteDeleted(out, req.EBID)
	}
	return nil
}

func (a *Agent) handleDeleteOne(ctx context.Context, req DeleteRequest) error {
	entry, err := a.catalog.Get(req.EBID)
	if err != nil {
		return fmt.Errorf("catalog get: %w", err)
	}
	if entry == nil {
		return fmt.Errorf("ebid not found: %s", req.EBID)
	}
	if err := a.provider.Delete(ctx, entry.ObjectKey); err != nil {
		// 允许后端返回"不存在"类错误后继续删除目录条目，避免孤立条目
		fmt.Fprintf(os.Stderr, "backint: storage delete warning for %s: %v\n", entry.ObjectKey, err)
	}
	return a.catalog.Delete(req.EBID)
}

// 辅助函数

func (a *Agent) objectKeyFor(ebid string) string {
	base := ebid + ".bin"
	if a.cfg.KeyPrefix == "" {
		return base
	}
	return path.Join(a.cfg.KeyPrefix, base)
}

// openBackupSource 打开 HANA 提供的数据源。
//
// 对于 #PIPE 模式：HANA 写入命名管道，Agent 读取。管道是顺序流，size 未知 (-1)。
// 对于文件模式：HANA 已在指定路径写好完整文件。
func openBackupSource(req BackupRequest) (io.ReadCloser, int64, error) {
	if req.IsPipe {
		f, err := os.OpenFile(req.Path, os.O_RDONLY, 0)
		if err != nil {
			return nil, 0, fmt.Errorf("open pipe: %w", err)
		}
		return f, -1, nil
	}
	f, err := os.Open(req.Path)
	if err != nil {
		return nil, 0, fmt.Errorf("open file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("stat: %w", err)
	}
	return f, info.Size(), nil
}

// openRestoreTarget 打开 HANA 指定的恢复目标（管道或文件）。
func openRestoreTarget(req RestoreRequest) (io.WriteCloser, error) {
	if req.IsPipe {
		return os.OpenFile(req.Path, os.O_WRONLY, 0)
	}
	return os.Create(req.Path)
}

// generateEBID 生成 Backint 外部备份 ID。
// 格式：backupx-<timestamp>-<16 hex chars>
func generateEBID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// fallback：用纳秒时间戳作为熵
		now := time.Now().UnixNano()
		for i := 0; i < 8; i++ {
			buf[i] = byte(now >> (i * 8))
		}
	}
	return fmt.Sprintf("backupx-%d-%s", time.Now().Unix(), hex.EncodeToString(buf[:]))
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// buildStorageRegistry 构造与主程序一致的 storage registry。
//
// Backint Agent 作为独立 CLI 进程运行，不依赖 BackupX HTTP 服务，
// 因此这里直接引用 storage/rclone 包注册所有后端。
func buildStorageRegistry() *storage.Registry {
	registry := storage.NewRegistry(
		storageRclone.NewLocalDiskFactory(),
		storageRclone.NewS3Factory(),
		storageRclone.NewWebDAVFactory(),
		storageRclone.NewGoogleDriveFactory(),
		storageRclone.NewAliyunOSSFactory(),
		storageRclone.NewTencentCOSFactory(),
		storageRclone.NewQiniuKodoFactory(),
		storageRclone.NewFTPFactory(),
		storageRclone.NewRcloneFactory(),
	)
	storageRclone.RegisterAllBackends(registry)
	return registry
}

