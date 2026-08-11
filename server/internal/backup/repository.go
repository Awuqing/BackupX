package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"backupx/server/internal/storage"
	"github.com/klauspost/compress/zstd"
)

const (
	repositoryFormatVersion = 1
	repositoryRoot          = ".backupx/repository/v1"
	repositoryIndexPrefix   = repositoryRoot + "/indexes"
	repositoryPackPrefix    = repositoryRoot + "/packs"
	repositorySnapshotRoot  = repositoryRoot + "/snapshots"
	repositoryDefaultPack   = int64(32 << 20)
	repositoryMaxIndexSize  = int64(64 << 20)
	repositoryMaxSnapshot   = int64(256 << 20)
	repositoryMaxEncoded    = int64(repositoryChunkMax + (256 << 10))
)

type RepositoryStore struct {
	key      []byte
	chunker  *contentDefinedChunker
	packSize int64
}

type RepositoryPlan struct {
	tempDir     string
	spoolPath   string
	chunks      map[string]repositoryPlanChunk
	chunkOrder  []string
	snapshot    repositorySnapshot
	Manifest    Manifest
	LogicalSize int64
	UniqueSize  int64
}

type repositoryPlanChunk struct {
	Offset int64
	Size   int64
}

type repositorySnapshot struct {
	Version     int               `json:"version"`
	TaskID      uint              `json:"taskId"`
	CreatedAt   time.Time         `json:"createdAt"`
	Compression string            `json:"compression"`
	Encrypted   bool              `json:"encrypted"`
	SourcePaths []string          `json:"sourcePaths"`
	Entries     []repositoryEntry `json:"entries"`
}

type repositoryEntry struct {
	Path       string   `json:"path"`
	Kind       string   `json:"kind"`
	Mode       uint32   `json:"mode"`
	ModTime    int64    `json:"modTime"`
	Size       int64    `json:"size"`
	LinkTarget string   `json:"linkTarget,omitempty"`
	Chunks     []string `json:"chunks,omitempty"`
}

type repositorySnapshotEnvelope struct {
	Version    int             `json:"version"`
	Encrypted  bool            `json:"encrypted"`
	Data       json.RawMessage `json:"data,omitempty"`
	Ciphertext string          `json:"ciphertext,omitempty"`
}

type repositoryChunkLocation struct {
	Pack        string `json:"pack"`
	Offset      int64  `json:"offset"`
	Length      int64  `json:"length"`
	PlainSize   int64  `json:"plainSize"`
	Compression string `json:"compression"`
	Encrypted   bool   `json:"encrypted"`
}

type repositoryIndexSegment struct {
	Version   int                                `json:"version"`
	CreatedAt time.Time                          `json:"createdAt"`
	Pack      string                             `json:"pack"`
	Chunks    map[string]repositoryChunkLocation `json:"chunks"`
}

type RepositoryUploadResult struct {
	SnapshotKey   string
	SnapshotSize  int64
	LogicalSize   int64
	UploadedBytes int64
	ReusedBytes   int64
	UniqueChunks  int
	NewChunks     int
	Checksum      string
}

type RepositoryVerifyResult struct {
	Entries int
	Chunks  int
	Bytes   int64
}

type RepositoryPruneResult struct {
	DeletedIndexes int
	DeletedPacks   int
	ReclaimedBytes int64
}

func NewRepositoryStore(encryptionKey []byte) *RepositoryStore {
	keyCopy := make([]byte, len(encryptionKey))
	copy(keyCopy, encryptionKey)
	return &RepositoryStore{
		key:      keyCopy,
		chunker:  newContentDefinedChunker(),
		packSize: repositoryDefaultPack,
	}
}

func (s *RepositoryStore) SnapshotKey(taskID, recordID uint, startedAt time.Time) string {
	stamp := startedAt.UTC().Format("20060102T150405.000000000Z")
	return fmt.Sprintf("%s/%d/%s-%d.bxrs", repositorySnapshotRoot, taskID, stamp, recordID)
}

func (p *RepositoryPlan) Close() error {
	if p == nil || strings.TrimSpace(p.tempDir) == "" {
		return nil
	}
	err := os.RemoveAll(p.tempDir)
	p.tempDir = ""
	p.spoolPath = ""
	return err
}

func (s *RepositoryStore) BuildPlan(ctx context.Context, task TaskSpec, writer LogWriter) (plan *RepositoryPlan, err error) {
	if writer == nil {
		writer = NopLogWriter{}
	}
	sourcePaths := compactPaths(task.SourcePaths)
	if len(sourcePaths) == 0 && strings.TrimSpace(task.SourcePath) != "" {
		sourcePaths = []string{filepath.Clean(strings.TrimSpace(task.SourcePath))}
	}
	if len(sourcePaths) == 0 {
		return nil, fmt.Errorf("source path is required")
	}
	compression, err := s.normalizeCompression(task.Compression)
	if err != nil {
		return nil, err
	}
	if task.Encrypt && len(s.key) != 32 {
		return nil, fmt.Errorf("repository encryption requires a 256-bit key")
	}
	if err := os.MkdirAll(task.TempDir, 0o755); err != nil {
		return nil, fmt.Errorf("create repository temp root: %w", err)
	}
	tempDir, err := os.MkdirTemp(task.TempDir, "repository-plan-*")
	if err != nil {
		return nil, fmt.Errorf("create repository plan directory: %w", err)
	}
	defer func() {
		if err == nil {
			return
		}
		if cleanupErr := os.RemoveAll(tempDir); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("clean repository plan: %w", cleanupErr))
		}
	}()

	spoolPath := filepath.Join(tempDir, "chunks.spool")
	spool, err := os.Create(spoolPath)
	if err != nil {
		return nil, fmt.Errorf("create repository chunk spool: %w", err)
	}
	spoolClosed := false
	defer func() {
		if spoolClosed {
			return
		}
		if closeErr := spool.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close repository chunk spool: %w", closeErr))
		}
	}()

	plan = &RepositoryPlan{
		tempDir:   tempDir,
		spoolPath: spoolPath,
		chunks:    make(map[string]repositoryPlanChunk),
		snapshot: repositorySnapshot{
			Version:     repositoryFormatVersion,
			TaskID:      task.ID,
			CreatedAt:   task.StartedAt.UTC(),
			Compression: compression,
			Encrypted:   task.Encrypt,
			SourcePaths: sourcePaths,
			Entries:     make([]repositoryEntry, 0),
		},
		Manifest: Manifest{Entries: make([]ManifestEntry, 0)},
	}
	excludes := normalizeExcludePatterns(task.ExcludePatterns)
	seenPaths := make(map[string]struct{})
	spoolOffset := int64(0)
	writer.WriteLine(fmt.Sprintf("CDC 仓库模式：FastCDC %d KiB/%d KiB/%d KiB，pack %d MiB", repositoryChunkMin>>10, repositoryChunkAvg>>10, repositoryChunkMax>>10, s.packSize>>20))

	for sourceIndex, rawSource := range sourcePaths {
		sourcePath := filepath.Clean(strings.TrimSpace(rawSource))
		if _, statErr := os.Lstat(sourcePath); statErr != nil {
			return nil, fmt.Errorf("stat source path %s: %w", sourcePath, statErr)
		}
		baseParent := filepath.Dir(sourcePath)
		writer.WriteLine(fmt.Sprintf("扫描源路径 [%d/%d]：%s", sourceIndex+1, len(sourcePaths), sourcePath))
		walkErr := filepath.Walk(sourcePath, func(currentPath string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			relative, relErr := filepath.Rel(baseParent, currentPath)
			if relErr != nil {
				return relErr
			}
			archiveName := path.Clean(filepath.ToSlash(relative))
			if archiveName == "." || strings.HasPrefix(archiveName, "../") {
				return fmt.Errorf("invalid repository entry path %q", archiveName)
			}
			if shouldExcludeEntry(archiveName, info.IsDir(), excludes) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if _, duplicated := seenPaths[archiveName]; duplicated {
				return fmt.Errorf("duplicate repository entry %q from overlapping source paths", archiveName)
			}
			seenPaths[archiveName] = struct{}{}

			entry := repositoryEntry{
				Path:    archiveName,
				Mode:    uint32(info.Mode().Perm()),
				ModTime: info.ModTime().UTC().UnixNano(),
				Size:    info.Size(),
			}
			manifestEntry := entryFromInfo(archiveName, info)
			switch {
			case info.IsDir():
				entry.Kind = "directory"
			case info.Mode()&os.ModeSymlink != 0:
				entry.Kind = "symlink"
				linkTarget, linkErr := os.Readlink(currentPath)
				if linkErr != nil {
					return fmt.Errorf("read symlink %s: %w", currentPath, linkErr)
				}
				entry.LinkTarget = linkTarget
			case info.Mode().IsRegular():
				entry.Kind = "file"
				plan.LogicalSize += info.Size()
				file, openErr := os.Open(currentPath)
				if openErr != nil {
					return fmt.Errorf("open source file %s: %w", currentPath, openErr)
				}
				splitErr := s.chunker.Split(ctx, file, func(raw []byte) error {
					chunkID := s.chunkID(raw, compression, task.Encrypt)
					entry.Chunks = append(entry.Chunks, chunkID)
					if _, exists := plan.chunks[chunkID]; exists {
						return nil
					}
					written, writeErr := spool.Write(raw)
					if writeErr != nil {
						return fmt.Errorf("write repository chunk spool: %w", writeErr)
					}
					if written != len(raw) {
						return io.ErrShortWrite
					}
					plan.chunks[chunkID] = repositoryPlanChunk{Offset: spoolOffset, Size: int64(len(raw))}
					plan.chunkOrder = append(plan.chunkOrder, chunkID)
					plan.UniqueSize += int64(len(raw))
					spoolOffset += int64(len(raw))
					return nil
				})
				closeErr := file.Close()
				if splitErr != nil {
					return errors.Join(fmt.Errorf("chunk source file %s: %w", currentPath, splitErr), closeErr)
				}
				if closeErr != nil {
					return fmt.Errorf("close source file %s: %w", currentPath, closeErr)
				}
			default:
				writer.WriteLine(fmt.Sprintf("跳过不支持的特殊文件：%s", currentPath))
				delete(seenPaths, archiveName)
				return nil
			}
			plan.snapshot.Entries = append(plan.snapshot.Entries, entry)
			plan.Manifest.Entries = append(plan.Manifest.Entries, manifestEntry)
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("scan source path %s: %w", sourcePath, walkErr)
		}
	}
	if err := spool.Sync(); err != nil {
		return nil, fmt.Errorf("sync repository chunk spool: %w", err)
	}
	if err := spool.Close(); err != nil {
		return nil, fmt.Errorf("close repository chunk spool: %w", err)
	}
	spoolClosed = true
	if err := s.validateSnapshot(&plan.snapshot); err != nil {
		return nil, err
	}
	writer.WriteLine(fmt.Sprintf("CDC 扫描完成：%d 个条目，逻辑数据 %d bytes，任务内唯一数据 %d bytes", len(plan.snapshot.Entries), plan.LogicalSize, plan.UniqueSize))
	return plan, nil
}

func (s *RepositoryStore) Upload(ctx context.Context, provider storage.StorageProvider, plan *RepositoryPlan, snapshotKey string) (*RepositoryUploadResult, error) {
	if provider == nil || plan == nil || strings.TrimSpace(snapshotKey) == "" {
		return nil, fmt.Errorf("repository provider, plan and snapshot key are required")
	}
	locations, err := s.loadIndex(ctx, provider)
	if err != nil {
		return nil, err
	}
	spool, err := os.Open(plan.spoolPath)
	if err != nil {
		return nil, fmt.Errorf("open repository chunk spool: %w", err)
	}
	uploadedBytes, newChunks, err := s.uploadMissingChunks(ctx, provider, plan, spool, locations)
	closeErr := spool.Close()
	if err != nil {
		return nil, errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close repository chunk spool: %w", closeErr)
	}

	snapshotBytes, err := s.encodeSnapshot(plan.snapshot)
	if err != nil {
		return nil, err
	}
	if err := provider.Upload(ctx, snapshotKey, bytes.NewReader(snapshotBytes), int64(len(snapshotBytes)), map[string]string{"format": "backupx-repository-v1"}); err != nil {
		return nil, fmt.Errorf("upload repository snapshot %s: %w", snapshotKey, err)
	}
	digest := sha256.Sum256(snapshotBytes)
	reusedBytes := int64(0)
	for _, chunkID := range plan.chunkOrder {
		if _, wasNew := newChunks[chunkID]; !wasNew {
			reusedBytes += plan.chunks[chunkID].Size
		}
	}
	return &RepositoryUploadResult{
		SnapshotKey:   snapshotKey,
		SnapshotSize:  int64(len(snapshotBytes)),
		LogicalSize:   plan.LogicalSize,
		UploadedBytes: uploadedBytes + int64(len(snapshotBytes)),
		ReusedBytes:   reusedBytes,
		UniqueChunks:  len(plan.chunkOrder),
		NewChunks:     len(newChunks),
		Checksum:      hex.EncodeToString(digest[:]),
	}, nil
}

// EstimateUploadSize returns a target-aware soft-quota estimate. Existing
// chunks are excluded so a nearly full repository can still accept a snapshot
// that is almost entirely deduplicated.
func (s *RepositoryStore) EstimateUploadSize(ctx context.Context, provider storage.StorageProvider, plan *RepositoryPlan) (int64, error) {
	if provider == nil || plan == nil {
		return 0, fmt.Errorf("repository provider and plan are required")
	}
	locations, err := s.loadIndex(ctx, provider)
	if err != nil {
		return 0, err
	}
	snapshotBytes, err := s.encodeSnapshot(plan.snapshot)
	if err != nil {
		return 0, err
	}
	estimate := int64(len(snapshotBytes))
	for _, chunkID := range plan.chunkOrder {
		if _, exists := locations[chunkID]; exists {
			continue
		}
		// Compression generally lowers this value. The small allowance covers
		// encryption tags and index metadata while keeping the check conservative.
		estimate += plan.chunks[chunkID].Size + 256
	}
	return estimate, nil
}

func (s *RepositoryStore) Restore(ctx context.Context, provider storage.StorageProvider, snapshotKey, expectedChecksum string, task TaskSpec, writer LogWriter) (err error) {
	if writer == nil {
		writer = NopLogWriter{}
	}
	if strings.TrimSpace(expectedChecksum) == "" {
		return fmt.Errorf("repository snapshot checksum is required for restore")
	}
	snapshot, _, err := s.loadSnapshot(ctx, provider, snapshotKey, expectedChecksum)
	if err != nil {
		return err
	}
	locations, err := s.loadIndex(ctx, provider)
	if err != nil {
		return err
	}
	restoreSource := strings.TrimSpace(task.SourcePath)
	if len(task.SourcePaths) > 0 {
		restoreSource = strings.TrimSpace(task.SourcePaths[0])
	}
	targetRoot := strings.TrimSpace(task.RestoreTargetPath)
	if targetRoot == "" {
		if restoreSource == "" {
			return fmt.Errorf("repository restore source path is required when no restore target is provided")
		}
		targetRoot = filepath.Dir(filepath.Clean(restoreSource))
	} else {
		targetRoot = filepath.Clean(targetRoot)
	}
	if !filepath.IsAbs(targetRoot) {
		return fmt.Errorf("repository restore target must be absolute: %s", targetRoot)
	}
	restoreRoot, err := s.openRestoreRoot(targetRoot)
	if err != nil {
		return fmt.Errorf("open repository restore root: %w", err)
	}
	defer func() {
		err = errors.Join(err, restoreRoot.Close())
	}()

	restored := 0
	directories := make([]repositoryEntry, 0)
	for _, entry := range snapshot.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if len(task.SelectedPaths) > 0 && !pathSelected(entry.Path, task.SelectedPaths) {
			continue
		}
		entryPath, localizeErr := filepath.Localize(entry.Path)
		if localizeErr != nil || !filepath.IsLocal(entryPath) {
			return fmt.Errorf("unsafe repository restore path %q", entry.Path)
		}
		if parent := filepath.Dir(entryPath); parent != "." {
			if err := restoreRoot.MkdirAll(parent, 0o755); err != nil {
				return fmt.Errorf("create restore parent for %s: %w", entry.Path, err)
			}
		}
		switch entry.Kind {
		case "directory":
			if info, statErr := restoreRoot.Lstat(entryPath); statErr == nil {
				if info.Mode()&os.ModeSymlink != 0 {
					return fmt.Errorf("restore directory crosses existing symlink: %s", entry.Path)
				}
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return fmt.Errorf("inspect restore directory %s: %w", entry.Path, statErr)
			}
			if err := restoreRoot.MkdirAll(entryPath, os.FileMode(entry.Mode)); err != nil {
				return fmt.Errorf("create restore directory %s: %w", entry.Path, err)
			}
			directories = append(directories, entry)
		case "symlink":
			resolvedLinkTarget := path.Clean(path.Join(path.Dir(entry.Path), strings.ReplaceAll(entry.LinkTarget, "\\", "/")))
			localizedLinkTarget, localizeTargetErr := filepath.Localize(resolvedLinkTarget)
			if localizeTargetErr != nil || !filepath.IsLocal(localizedLinkTarget) {
				return fmt.Errorf("unsafe repository symlink target %q", entry.LinkTarget)
			}
			linkTarget, relativeTargetErr := filepath.Rel(filepath.Dir(entryPath), localizedLinkTarget)
			if relativeTargetErr != nil {
				return fmt.Errorf("resolve restore symlink target %s: %w", entry.Path, relativeTargetErr)
			}
			if info, statErr := restoreRoot.Lstat(entryPath); statErr == nil {
				if info.IsDir() {
					return fmt.Errorf("refuse to replace restore directory with symlink: %s", entry.Path)
				}
				if err := restoreRoot.Remove(entryPath); err != nil {
					return fmt.Errorf("replace restore symlink %s: %w", entry.Path, err)
				}
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return fmt.Errorf("inspect restore symlink %s: %w", entry.Path, statErr)
			}
			if err := restoreRoot.Symlink(linkTarget, entryPath); err != nil {
				return fmt.Errorf("create restore symlink %s: %w", entry.Path, err)
			}
		case "file":
			if info, statErr := restoreRoot.Lstat(entryPath); statErr == nil {
				if info.Mode()&os.ModeSymlink != 0 {
					return fmt.Errorf("restore file would overwrite existing symlink: %s", entry.Path)
				}
			} else if !errors.Is(statErr, os.ErrNotExist) {
				return fmt.Errorf("inspect restore file %s: %w", entry.Path, statErr)
			}
			file, openErr := restoreRoot.OpenFile(entryPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(entry.Mode))
			if openErr != nil {
				return fmt.Errorf("create restore file %s: %w", entry.Path, openErr)
			}
			var written int64
			for _, chunkID := range entry.Chunks {
				raw, readErr := s.readChunk(ctx, provider, chunkID, locations)
				if readErr != nil {
					return errors.Join(fmt.Errorf("restore %s: %w", entry.Path, readErr), file.Close())
				}
				count, writeErr := file.Write(raw)
				written += int64(count)
				if writeErr != nil {
					return errors.Join(fmt.Errorf("write restore file %s: %w", entry.Path, writeErr), file.Close())
				}
				if count != len(raw) {
					return errors.Join(io.ErrShortWrite, file.Close())
				}
			}
			if written != entry.Size {
				return errors.Join(fmt.Errorf("restored size mismatch for %s: expected %d, got %d", entry.Path, entry.Size, written), file.Close())
			}
			if err := file.Chmod(os.FileMode(entry.Mode)); err != nil {
				return errors.Join(fmt.Errorf("restore mode for %s: %w", entry.Path, err), file.Close())
			}
			if closeErr := file.Close(); closeErr != nil {
				return fmt.Errorf("close restore file %s: %w", entry.Path, closeErr)
			}
			modTime := time.Unix(0, entry.ModTime)
			if err := restoreRoot.Chtimes(entryPath, modTime, modTime); err != nil {
				return fmt.Errorf("restore timestamp for %s: %w", entry.Path, err)
			}
		default:
			return fmt.Errorf("unsupported repository entry kind %q", entry.Kind)
		}
		restored++
	}
	// Children modify their parent directory timestamps, so directory metadata
	// is restored only after every selected file has been written.
	for index := len(directories) - 1; index >= 0; index-- {
		entry := directories[index]
		entryPath, localizeErr := filepath.Localize(entry.Path)
		if localizeErr != nil || !filepath.IsLocal(entryPath) {
			return fmt.Errorf("unsafe repository restore path %q", entry.Path)
		}
		if err := restoreRoot.Chmod(entryPath, os.FileMode(entry.Mode)); err != nil {
			return fmt.Errorf("restore mode for %s: %w", entry.Path, err)
		}
		modTime := time.Unix(0, entry.ModTime)
		if err := restoreRoot.Chtimes(entryPath, modTime, modTime); err != nil {
			return fmt.Errorf("restore timestamp for %s: %w", entry.Path, err)
		}
	}
	writer.WriteLine(fmt.Sprintf("CDC 仓库恢复完成：%d 个条目", restored))
	return nil
}

func (s *RepositoryStore) ExportTar(ctx context.Context, provider storage.StorageProvider, snapshotKey, expectedChecksum, destination string) (err error) {
	if strings.TrimSpace(expectedChecksum) == "" {
		return fmt.Errorf("repository snapshot checksum is required for export")
	}
	snapshot, _, err := s.loadSnapshot(ctx, provider, snapshotKey, expectedChecksum)
	if err != nil {
		return err
	}
	locations, err := s.loadIndex(ctx, provider)
	if err != nil {
		return err
	}
	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("create repository export: %w", err)
	}
	tw := tar.NewWriter(file)
	tarClosed := false
	fileClosed := false
	defer func() {
		if !tarClosed {
			err = errors.Join(err, tw.Close())
		}
		if !fileClosed {
			err = errors.Join(err, file.Close())
		}
	}()
	for _, entry := range snapshot.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		header := &tar.Header{
			Name:    entry.Path,
			Mode:    int64(entry.Mode),
			ModTime: time.Unix(0, entry.ModTime),
			Size:    entry.Size,
		}
		switch entry.Kind {
		case "directory":
			header.Typeflag = tar.TypeDir
			header.Size = 0
			header.Name = strings.TrimSuffix(entry.Path, "/") + "/"
		case "symlink":
			header.Typeflag = tar.TypeSymlink
			header.Size = 0
			header.Linkname = entry.LinkTarget
		case "file":
			header.Typeflag = tar.TypeReg
		default:
			return fmt.Errorf("unsupported repository entry kind %q", entry.Kind)
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write repository tar header: %w", err)
		}
		if entry.Kind == "file" {
			for _, chunkID := range entry.Chunks {
				raw, readErr := s.readChunk(ctx, provider, chunkID, locations)
				if readErr != nil {
					return readErr
				}
				if _, writeErr := tw.Write(raw); writeErr != nil {
					return fmt.Errorf("write repository tar data: %w", writeErr)
				}
			}
		}
	}
	closeErr := tw.Close()
	tarClosed = true
	if closeErr != nil {
		return fmt.Errorf("close repository tar: %w", closeErr)
	}
	closeErr = file.Close()
	fileClosed = true
	if closeErr != nil {
		return fmt.Errorf("close repository export: %w", closeErr)
	}
	return nil
}

func (s *RepositoryStore) Verify(ctx context.Context, provider storage.StorageProvider, snapshotKey, expectedChecksum string) (*RepositoryVerifyResult, error) {
	if strings.TrimSpace(expectedChecksum) == "" {
		return nil, fmt.Errorf("repository snapshot checksum is required for verification")
	}
	snapshot, _, err := s.loadSnapshot(ctx, provider, snapshotKey, expectedChecksum)
	if err != nil {
		return nil, err
	}
	locations, err := s.loadIndex(ctx, provider)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	result := &RepositoryVerifyResult{Entries: len(snapshot.Entries)}
	for _, entry := range snapshot.Entries {
		for _, chunkID := range entry.Chunks {
			if _, checked := seen[chunkID]; checked {
				continue
			}
			raw, readErr := s.readChunk(ctx, provider, chunkID, locations)
			if readErr != nil {
				return nil, readErr
			}
			seen[chunkID] = struct{}{}
			result.Chunks++
			result.Bytes += int64(len(raw))
		}
	}
	return result, nil
}

func (s *RepositoryStore) Prune(ctx context.Context, provider storage.StorageProvider) (*RepositoryPruneResult, error) {
	if provider == nil {
		return nil, fmt.Errorf("repository provider is required")
	}
	liveChunks := make(map[string]struct{})
	snapshots, err := provider.List(ctx, repositorySnapshotRoot)
	if err != nil {
		return nil, fmt.Errorf("list repository snapshots: %w", err)
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Key < snapshots[j].Key })
	for _, object := range snapshots {
		snapshot, _, loadErr := s.loadSnapshot(ctx, provider, object.Key, "")
		if loadErr != nil {
			return nil, fmt.Errorf("refuse to prune with unreadable snapshot %s: %w", object.Key, loadErr)
		}
		for _, entry := range snapshot.Entries {
			for _, chunkID := range entry.Chunks {
				liveChunks[chunkID] = struct{}{}
			}
		}
	}

	indexObjects, err := provider.List(ctx, repositoryIndexPrefix)
	if err != nil {
		return nil, fmt.Errorf("list repository indexes: %w", err)
	}
	packSizes := make(map[string]int64)
	packObjects, err := provider.List(ctx, repositoryPackPrefix)
	if err != nil {
		return nil, fmt.Errorf("list repository packs: %w", err)
	}
	for _, object := range packObjects {
		packSizes[object.Key] = object.Size
	}
	keptPacks := make(map[string]struct{})
	result := &RepositoryPruneResult{}
	for _, object := range indexObjects {
		segment, readErr := s.readIndexSegment(ctx, provider, object.Key)
		if readErr != nil {
			return nil, fmt.Errorf("refuse to prune with unreadable index %s: %w", object.Key, readErr)
		}
		keep := false
		for chunkID := range segment.Chunks {
			if _, live := liveChunks[chunkID]; live {
				keep = true
				break
			}
		}
		if keep {
			keptPacks[segment.Pack] = struct{}{}
			continue
		}
		if err := provider.Delete(ctx, object.Key); err != nil {
			return nil, fmt.Errorf("delete unused repository index %s: %w", object.Key, err)
		}
		result.DeletedIndexes++
		if err := provider.Delete(ctx, segment.Pack); err != nil {
			return nil, fmt.Errorf("delete unused repository pack %s: %w", segment.Pack, err)
		}
		result.DeletedPacks++
		result.ReclaimedBytes += packSizes[segment.Pack]
		delete(packSizes, segment.Pack)
	}
	for packKey, size := range packSizes {
		if _, keep := keptPacks[packKey]; keep {
			continue
		}
		if err := provider.Delete(ctx, packKey); err != nil {
			return nil, fmt.Errorf("delete orphaned repository pack %s: %w", packKey, err)
		}
		result.DeletedPacks++
		result.ReclaimedBytes += size
	}
	return result, nil
}

func (s *RepositoryStore) uploadMissingChunks(ctx context.Context, provider storage.StorageProvider, plan *RepositoryPlan, spool *os.File, locations map[string]repositoryChunkLocation) (uploadedBytes int64, newChunkIDs map[string]struct{}, err error) {
	newChunkIDs = make(map[string]struct{})
	packPath := ""
	var packFile *os.File
	packSize := int64(0)
	packChunks := make(map[string]repositoryChunkLocation)

	closeAndRemove := func() error {
		var cleanupErr error
		if packFile != nil {
			cleanupErr = packFile.Close()
			packFile = nil
		}
		if packPath != "" {
			cleanupErr = errors.Join(cleanupErr, os.Remove(packPath))
			packPath = ""
		}
		return cleanupErr
	}
	flush := func() error {
		if packFile == nil || len(packChunks) == 0 {
			return nil
		}
		if err := packFile.Sync(); err != nil {
			return fmt.Errorf("sync repository pack: %w", err)
		}
		if err := packFile.Close(); err != nil {
			return fmt.Errorf("close repository pack: %w", err)
		}
		packFile = nil
		packKey, indexBytes, packBytes, err := s.uploadPack(ctx, provider, packPath, packChunks)
		if err != nil {
			return err
		}
		for chunkID, location := range packChunks {
			location.Pack = packKey
			locations[chunkID] = location
			newChunkIDs[chunkID] = struct{}{}
		}
		uploadedBytes += packBytes + indexBytes
		if err := os.Remove(packPath); err != nil {
			return fmt.Errorf("remove temporary repository pack: %w", err)
		}
		packPath = ""
		packSize = 0
		packChunks = make(map[string]repositoryChunkLocation)
		return nil
	}
	defer func() {
		if cleanupErr := closeAndRemove(); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("clean temporary repository pack: %w", cleanupErr))
		}
	}()

	for _, chunkID := range plan.chunkOrder {
		if _, exists := locations[chunkID]; exists {
			continue
		}
		planChunk := plan.chunks[chunkID]
		raw := make([]byte, planChunk.Size)
		readCount, readErr := spool.ReadAt(raw, planChunk.Offset)
		if readErr != nil && readErr != io.EOF {
			return 0, nil, fmt.Errorf("read repository chunk spool: %w", readErr)
		}
		if int64(readCount) != planChunk.Size {
			return 0, nil, io.ErrUnexpectedEOF
		}
		encoded, encodeErr := s.encodeChunk(raw, plan.snapshot.Compression, plan.snapshot.Encrypted, chunkID)
		if encodeErr != nil {
			return 0, nil, encodeErr
		}
		if packFile != nil && packSize > 0 && packSize+int64(len(encoded)) > s.packSize {
			if err := flush(); err != nil {
				return 0, nil, err
			}
		}
		if packFile == nil {
			created, createErr := os.CreateTemp(plan.tempDir, "repository-pack-*")
			if createErr != nil {
				return 0, nil, fmt.Errorf("create temporary repository pack: %w", createErr)
			}
			packFile = created
			packPath = created.Name()
		}
		written, writeErr := packFile.Write(encoded)
		if writeErr != nil {
			return 0, nil, fmt.Errorf("write repository pack: %w", writeErr)
		}
		if written != len(encoded) {
			return 0, nil, io.ErrShortWrite
		}
		packChunks[chunkID] = repositoryChunkLocation{
			Offset:      packSize,
			Length:      int64(len(encoded)),
			PlainSize:   planChunk.Size,
			Compression: plan.snapshot.Compression,
			Encrypted:   plan.snapshot.Encrypted,
		}
		packSize += int64(len(encoded))
	}
	if err := flush(); err != nil {
		return 0, nil, err
	}
	return uploadedBytes, newChunkIDs, nil
}

func (s *RepositoryStore) uploadPack(ctx context.Context, provider storage.StorageProvider, packPath string, chunks map[string]repositoryChunkLocation) (packKey string, indexSize int64, packSize int64, err error) {
	pack, err := os.Open(packPath)
	if err != nil {
		return "", 0, 0, fmt.Errorf("open repository pack: %w", err)
	}
	packClosed := false
	defer func() {
		if !packClosed {
			err = errors.Join(err, pack.Close())
		}
	}()
	hasher := sha256.New()
	packBytes, err := io.Copy(hasher, pack)
	if err != nil {
		return "", 0, 0, fmt.Errorf("hash repository pack: %w", err)
	}
	packID := hex.EncodeToString(hasher.Sum(nil))
	packKey = fmt.Sprintf("%s/%s/%s.pack", repositoryPackPrefix, packID[:2], packID)
	for chunkID, location := range chunks {
		location.Pack = packKey
		chunks[chunkID] = location
	}
	segment := repositoryIndexSegment{Version: repositoryFormatVersion, CreatedAt: time.Now().UTC(), Pack: packKey, Chunks: chunks}
	indexBytes, err := json.Marshal(segment)
	if err != nil {
		return "", 0, 0, fmt.Errorf("encode repository index: %w", err)
	}
	if _, err := pack.Seek(0, io.SeekStart); err != nil {
		return "", 0, 0, fmt.Errorf("rewind repository pack: %w", err)
	}
	if err := provider.Upload(ctx, packKey, pack, packBytes, map[string]string{"format": "backupx-pack-v1"}); err != nil {
		return "", 0, 0, fmt.Errorf("upload repository pack %s: %w", packKey, err)
	}
	closeErr := pack.Close()
	packClosed = true
	if closeErr != nil {
		return "", 0, 0, fmt.Errorf("close repository pack: %w", closeErr)
	}
	indexKey := fmt.Sprintf("%s/%s.json", repositoryIndexPrefix, packID)
	if err := provider.Upload(ctx, indexKey, bytes.NewReader(indexBytes), int64(len(indexBytes)), map[string]string{"format": "backupx-index-v1"}); err != nil {
		deleteErr := provider.Delete(ctx, packKey)
		return "", 0, 0, errors.Join(fmt.Errorf("upload repository index %s: %w", indexKey, err), deleteErr)
	}
	return packKey, int64(len(indexBytes)), packBytes, nil
}

func (s *RepositoryStore) loadIndex(ctx context.Context, provider storage.StorageProvider) (map[string]repositoryChunkLocation, error) {
	objects, err := provider.List(ctx, repositoryIndexPrefix)
	if err != nil {
		return nil, fmt.Errorf("list repository indexes: %w", err)
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].Key < objects[j].Key })
	locations := make(map[string]repositoryChunkLocation)
	for _, object := range objects {
		segment, readErr := s.readIndexSegment(ctx, provider, object.Key)
		if readErr != nil {
			return nil, readErr
		}
		for chunkID, location := range segment.Chunks {
			if _, exists := locations[chunkID]; !exists {
				locations[chunkID] = location
			}
		}
	}
	return locations, nil
}

func (s *RepositoryStore) readIndexSegment(ctx context.Context, provider storage.StorageProvider, key string) (*repositoryIndexSegment, error) {
	indexPrefix := repositoryIndexPrefix + "/"
	indexName := strings.TrimPrefix(key, indexPrefix)
	indexID := strings.TrimSuffix(indexName, ".json")
	decodedIndexID, decodeIndexErr := hex.DecodeString(indexID)
	if indexName == key || indexID == indexName || indexID != strings.ToLower(indexID) || strings.Contains(indexName, "/") || strings.Contains(indexName, "\\") || decodeIndexErr != nil || len(decodedIndexID) != sha256.Size {
		return nil, fmt.Errorf("invalid repository index key %q", key)
	}
	expectedPack := fmt.Sprintf("%s/%s/%s.pack", repositoryPackPrefix, indexID[:2], indexID)
	reader, err := provider.Download(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("download repository index %s: %w", key, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, repositoryMaxIndexSize+1))
	closeErr := reader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read repository index %s: %w", key, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close repository index %s: %w", key, closeErr)
	}
	if int64(len(data)) > repositoryMaxIndexSize {
		return nil, fmt.Errorf("repository index %s exceeds %d bytes", key, repositoryMaxIndexSize)
	}
	var segment repositoryIndexSegment
	if err := json.Unmarshal(data, &segment); err != nil {
		return nil, fmt.Errorf("decode repository index %s: %w", key, err)
	}
	if segment.Version != repositoryFormatVersion || segment.Pack != expectedPack || len(segment.Chunks) == 0 {
		return nil, fmt.Errorf("unsupported repository index %s", key)
	}
	packLimit := s.packSize
	if packLimit <= 0 {
		packLimit = repositoryDefaultPack
	}
	if packLimit < repositoryMaxEncoded {
		packLimit = repositoryMaxEncoded
	}
	for chunkID, location := range segment.Chunks {
		expectedPrefix := "p-"
		if location.Encrypted {
			expectedPrefix = "e-"
		}
		encodedID := strings.TrimPrefix(chunkID, expectedPrefix)
		decodedID, decodeErr := hex.DecodeString(encodedID)
		validPrefix := encodedID != chunkID
		validCompression := location.Compression == "none" || location.Compression == "gzip" || location.Compression == "zstd"
		if len(decodedID) != sha256.Size || decodeErr != nil || encodedID != strings.ToLower(encodedID) || !validPrefix || !validCompression || location.Pack != expectedPack || location.Offset < 0 || location.Length <= 0 || location.Length > repositoryMaxEncoded || location.Length > packLimit || location.PlainSize <= 0 || location.PlainSize > repositoryChunkMax || location.Offset > packLimit-location.Length {
			return nil, fmt.Errorf("invalid chunk location in repository index %s", key)
		}
	}
	return &segment, nil
}

func (s *RepositoryStore) loadSnapshot(ctx context.Context, provider storage.StorageProvider, key, expectedChecksum string) (*repositorySnapshot, []byte, error) {
	reader, err := provider.Download(ctx, key)
	if err != nil {
		return nil, nil, fmt.Errorf("download repository snapshot %s: %w", key, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, repositoryMaxSnapshot+1))
	closeErr := reader.Close()
	if readErr != nil {
		return nil, nil, fmt.Errorf("read repository snapshot %s: %w", key, readErr)
	}
	if closeErr != nil {
		return nil, nil, fmt.Errorf("close repository snapshot %s: %w", key, closeErr)
	}
	if int64(len(data)) > repositoryMaxSnapshot {
		return nil, nil, fmt.Errorf("repository snapshot %s exceeds %d bytes", key, repositoryMaxSnapshot)
	}
	if expected := strings.TrimSpace(expectedChecksum); expected != "" {
		expectedBytes, decodeErr := hex.DecodeString(expected)
		if decodeErr != nil || len(expectedBytes) != sha256.Size {
			return nil, nil, fmt.Errorf("repository snapshot checksum is invalid")
		}
		digest := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(digest[:]), expected) {
			return nil, nil, fmt.Errorf("repository snapshot checksum mismatch")
		}
	}
	var envelope repositorySnapshotEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, nil, fmt.Errorf("decode repository snapshot envelope %s: %w", key, err)
	}
	if envelope.Version != repositoryFormatVersion {
		return nil, nil, fmt.Errorf("unsupported repository snapshot version %d", envelope.Version)
	}
	payload := []byte(envelope.Data)
	if envelope.Encrypted {
		ciphertext, decodeErr := base64.RawURLEncoding.DecodeString(envelope.Ciphertext)
		if decodeErr != nil {
			return nil, nil, fmt.Errorf("decode repository snapshot ciphertext: %w", decodeErr)
		}
		payload, err = s.decrypt(ciphertext, []byte("backupx-repository-snapshot-v1"))
		if err != nil {
			return nil, nil, fmt.Errorf("decrypt repository snapshot: %w", err)
		}
	}
	var snapshot repositorySnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return nil, nil, fmt.Errorf("decode repository snapshot: %w", err)
	}
	if snapshot.Version != repositoryFormatVersion {
		return nil, nil, fmt.Errorf("unsupported repository snapshot payload version %d", snapshot.Version)
	}
	if snapshot.Encrypted != envelope.Encrypted {
		return nil, nil, fmt.Errorf("repository snapshot encryption metadata mismatch")
	}
	if err := s.validateSnapshot(&snapshot); err != nil {
		return nil, nil, err
	}
	return &snapshot, data, nil
}

func (*RepositoryStore) validateSnapshot(snapshot *repositorySnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("repository snapshot is required")
	}
	if snapshot.Version != repositoryFormatVersion {
		return fmt.Errorf("unsupported repository snapshot payload version %d", snapshot.Version)
	}
	if snapshot.Compression != "none" && snapshot.Compression != "gzip" && snapshot.Compression != "zstd" {
		return fmt.Errorf("unsupported repository snapshot compression %q", snapshot.Compression)
	}
	seenPaths := make(map[string]struct{}, len(snapshot.Entries))
	symlinkPaths := make(map[string]struct{})
	for _, entry := range snapshot.Entries {
		if entry.Path == "." || !fs.ValidPath(entry.Path) || path.Clean(entry.Path) != entry.Path || strings.Contains(entry.Path, "\\") || entry.Mode & ^uint32(0o777) != 0 || entry.Size < 0 {
			return fmt.Errorf("invalid repository snapshot entry %q", entry.Path)
		}
		if _, exists := seenPaths[entry.Path]; exists {
			return fmt.Errorf("duplicate repository snapshot entry %q", entry.Path)
		}
		seenPaths[entry.Path] = struct{}{}
		switch entry.Kind {
		case "directory":
			if len(entry.Chunks) != 0 || entry.LinkTarget != "" {
				return fmt.Errorf("invalid repository directory entry %q", entry.Path)
			}
		case "symlink":
			if entry.LinkTarget == "" || len(entry.Chunks) != 0 {
				return fmt.Errorf("invalid repository symlink entry %q", entry.Path)
			}
			normalizedTarget := strings.ReplaceAll(entry.LinkTarget, "\\", "/")
			resolvedTarget := path.Clean(path.Join(path.Dir(entry.Path), normalizedTarget))
			if strings.ContainsRune(entry.LinkTarget, 0) || path.IsAbs(normalizedTarget) || filepath.IsAbs(entry.LinkTarget) || filepath.VolumeName(entry.LinkTarget) != "" || resolvedTarget == ".." || strings.HasPrefix(resolvedTarget, "../") {
				return fmt.Errorf("repository symlink %q escapes the restore root", entry.Path)
			}
			symlinkPaths[entry.Path] = struct{}{}
		case "file":
			if entry.LinkTarget != "" || (entry.Size == 0 && len(entry.Chunks) != 0) || (entry.Size > 0 && len(entry.Chunks) == 0) {
				return fmt.Errorf("invalid repository file entry %q", entry.Path)
			}
			expectedPrefix := "p-"
			if snapshot.Encrypted {
				expectedPrefix = "e-"
			}
			for _, chunkID := range entry.Chunks {
				encodedID := strings.TrimPrefix(chunkID, expectedPrefix)
				decodedID, decodeErr := hex.DecodeString(encodedID)
				if decodeErr != nil || len(decodedID) != sha256.Size || encodedID == chunkID || encodedID != strings.ToLower(encodedID) {
					return fmt.Errorf("invalid chunk id in repository entry %q", entry.Path)
				}
			}
		default:
			return fmt.Errorf("unsupported repository entry kind %q", entry.Kind)
		}
	}
	for entryPath := range seenPaths {
		for parent := path.Dir(entryPath); parent != "."; parent = path.Dir(parent) {
			if _, crossesSymlink := symlinkPaths[parent]; crossesSymlink {
				return fmt.Errorf("repository entry %q crosses symlink %q", entryPath, parent)
			}
		}
	}
	return nil
}

func (s *RepositoryStore) encodeSnapshot(snapshot repositorySnapshot) ([]byte, error) {
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode repository snapshot: %w", err)
	}
	envelope := repositorySnapshotEnvelope{Version: repositoryFormatVersion, Encrypted: snapshot.Encrypted}
	if snapshot.Encrypted {
		ciphertext, encryptErr := s.encrypt(payload, []byte("backupx-repository-snapshot-v1"))
		if encryptErr != nil {
			return nil, fmt.Errorf("encrypt repository snapshot: %w", encryptErr)
		}
		envelope.Ciphertext = base64.RawURLEncoding.EncodeToString(ciphertext)
	} else {
		envelope.Data = payload
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode repository snapshot envelope: %w", err)
	}
	return data, nil
}

func (s *RepositoryStore) readChunk(ctx context.Context, provider storage.StorageProvider, chunkID string, locations map[string]repositoryChunkLocation) ([]byte, error) {
	location, exists := locations[chunkID]
	if !exists {
		return nil, fmt.Errorf("repository chunk %s is missing from the index", chunkID)
	}
	var reader io.ReadCloser
	var err error
	if ranged, ok := provider.(storage.StorageRangeDownloader); ok {
		reader, err = ranged.DownloadRange(ctx, location.Pack, location.Offset, location.Length)
	} else {
		reader, err = provider.Download(ctx, location.Pack)
		if err == nil && location.Offset > 0 {
			if _, copyErr := io.CopyN(io.Discard, reader, location.Offset); copyErr != nil {
				return nil, errors.Join(fmt.Errorf("seek repository pack %s: %w", location.Pack, copyErr), reader.Close())
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("read repository pack %s: %w", location.Pack, err)
	}
	encoded := make([]byte, int(location.Length))
	_, readErr := io.ReadFull(reader, encoded)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read repository chunk %s: %w", chunkID, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close repository pack %s: %w", location.Pack, closeErr)
	}
	raw, err := s.decodeChunk(encoded, location, chunkID)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) != location.PlainSize {
		return nil, fmt.Errorf("repository chunk %s size mismatch", chunkID)
	}
	if actual := s.chunkID(raw, location.Compression, location.Encrypted); actual != chunkID {
		return nil, fmt.Errorf("repository chunk %s failed content verification", chunkID)
	}
	return raw, nil
}

func (s *RepositoryStore) encodeChunk(raw []byte, compression string, encrypted bool, chunkID string) ([]byte, error) {
	var encoded []byte
	switch compression {
	case "none":
		encoded = append([]byte(nil), raw...)
	case "gzip":
		var buffer bytes.Buffer
		writer := gzip.NewWriter(&buffer)
		if _, err := writer.Write(raw); err != nil {
			return nil, errors.Join(fmt.Errorf("gzip repository chunk: %w", err), writer.Close())
		}
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("close repository gzip chunk: %w", err)
		}
		encoded = buffer.Bytes()
	case "zstd":
		writer, err := zstd.NewWriter(nil)
		if err != nil {
			return nil, fmt.Errorf("create repository zstd encoder: %w", err)
		}
		encoded = writer.EncodeAll(raw, nil)
		writer.Close()
	default:
		return nil, fmt.Errorf("unsupported repository compression %q", compression)
	}
	if !encrypted {
		return encoded, nil
	}
	ciphertext, err := s.encrypt(encoded, []byte(chunkID))
	if err != nil {
		return nil, fmt.Errorf("encrypt repository chunk: %w", err)
	}
	return ciphertext, nil
}

func (s *RepositoryStore) decodeChunk(encoded []byte, location repositoryChunkLocation, chunkID string) ([]byte, error) {
	payload := encoded
	var err error
	if location.Encrypted {
		payload, err = s.decrypt(encoded, []byte(chunkID))
		if err != nil {
			return nil, fmt.Errorf("decrypt repository chunk %s: %w", chunkID, err)
		}
	}
	switch location.Compression {
	case "none":
		return payload, nil
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("open repository gzip chunk: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(reader, location.PlainSize+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("decompress repository gzip chunk: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close repository gzip chunk: %w", closeErr)
		}
		return raw, nil
	case "zstd":
		reader, err := zstd.NewReader(bytes.NewReader(payload), zstd.WithDecoderMaxMemory(uint64(repositoryChunkMax+(1<<20))))
		if err != nil {
			return nil, fmt.Errorf("create repository zstd decoder: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(reader, location.PlainSize+1))
		reader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("decompress repository zstd chunk: %w", readErr)
		}
		return raw, nil
	default:
		return nil, fmt.Errorf("unsupported repository compression %q", location.Compression)
	}
}

func (s *RepositoryStore) chunkID(raw []byte, compression string, encrypted bool) string {
	domain := fmt.Sprintf("backupx-repository-v1/%s/plain\x00", compression)
	prefix := "p-"
	if encrypted {
		domain = fmt.Sprintf("backupx-repository-v1/%s/encrypted\x00", compression)
		mac := hmac.New(sha256.New, s.key)
		mac.Write([]byte(domain))
		mac.Write(raw)
		return "e-" + hex.EncodeToString(mac.Sum(nil))
	}
	digest := sha256.New()
	digest.Write([]byte(domain))
	digest.Write(raw)
	return prefix + hex.EncodeToString(digest.Sum(nil))
}

func (s *RepositoryStore) encrypt(plain, additionalData []byte) ([]byte, error) {
	if len(s.key) != 32 {
		return nil, fmt.Errorf("repository encryption key is unavailable")
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, additionalData), nil
}

func (s *RepositoryStore) decrypt(ciphertext, additionalData []byte) ([]byte, error) {
	if len(s.key) != 32 {
		return nil, fmt.Errorf("repository encryption key is unavailable")
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("repository ciphertext is too short")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	return gcm.Open(nil, nonce, ciphertext[gcm.NonceSize():], additionalData)
}

func (s *RepositoryStore) normalizeCompression(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "zstd":
		return "zstd", nil
	case "gzip":
		return "gzip", nil
	case "none":
		return "none", nil
	default:
		return "", fmt.Errorf("unsupported repository compression %q", value)
	}
}

func (s *RepositoryStore) openRestoreRoot(target string) (*os.Root, error) {
	cleanTarget := filepath.Clean(target)
	if !filepath.IsAbs(cleanTarget) {
		return nil, fmt.Errorf("restore target must be absolute: %s", target)
	}
	volume := filepath.VolumeName(cleanTarget)
	if strings.Contains(volume, "..") || strings.ContainsRune(volume, 0) {
		return nil, fmt.Errorf("invalid restore target volume: %s", target)
	}
	volumeRootPath := string(filepath.Separator)
	relativeTarget := strings.TrimLeft(cleanTarget, string(filepath.Separator))
	if volume != "" {
		volumeRootPath = volume + string(filepath.Separator)
		relativeTarget = strings.TrimLeft(strings.TrimPrefix(cleanTarget, volume), string(filepath.Separator))
	}
	if relativeTarget != "" && !filepath.IsLocal(relativeTarget) {
		return nil, fmt.Errorf("restore target is not local to its volume: %s", target)
	}
	volumeRoot, err := os.OpenRoot(volumeRootPath)
	if err != nil {
		return nil, fmt.Errorf("open restore volume: %w", err)
	}
	if relativeTarget == "" {
		return volumeRoot, nil
	}
	if err := volumeRoot.MkdirAll(relativeTarget, 0o755); err != nil {
		return nil, errors.Join(fmt.Errorf("create repository restore root: %w", err), volumeRoot.Close())
	}
	restoreRoot, openErr := volumeRoot.OpenRoot(relativeTarget)
	closeErr := volumeRoot.Close()
	if openErr != nil || closeErr != nil {
		return nil, errors.Join(openErr, closeErr)
	}
	return restoreRoot, nil
}

func compactPaths(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if value := strings.TrimSpace(item); value != "" {
			result = append(result, filepath.Clean(value))
		}
	}
	return result
}
