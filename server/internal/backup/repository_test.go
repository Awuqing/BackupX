package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"backupx/server/internal/storage"
)

func TestContentDefinedChunkerResynchronizesAfterInsertion(t *testing.T) {
	source := make([]byte, 8<<20)
	if _, err := rand.New(rand.NewSource(42)).Read(source); err != nil {
		t.Fatalf("generate source: %v", err)
	}
	modified := make([]byte, 0, len(source)+4096)
	modified = append(modified, source[:2<<20]...)
	modified = append(modified, bytes.Repeat([]byte("inserted"), 512)...)
	modified = append(modified, source[2<<20:]...)

	chunker := newContentDefinedChunker()
	collect := func(data []byte) map[string]struct{} {
		t.Helper()
		ids := make(map[string]struct{})
		err := chunker.Split(context.Background(), bytes.NewReader(data), func(chunk []byte) error {
			digest := sha256.Sum256(chunk)
			ids[fmt.Sprintf("%x", digest[:])] = struct{}{}
			if len(chunk) > repositoryChunkMax {
				return fmt.Errorf("chunk exceeds maximum: %d", len(chunk))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("split chunks: %v", err)
		}
		return ids
	}
	originalChunks := collect(source)
	modifiedChunks := collect(modified)
	shared := 0
	for chunkID := range originalChunks {
		if _, ok := modifiedChunks[chunkID]; ok {
			shared++
		}
	}
	if shared < len(originalChunks)/2 {
		t.Fatalf("content-defined boundaries did not resynchronize: shared=%d original=%d", shared, len(originalChunks))
	}
}

func TestRepositoryRoundTripDedupAndPrune(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "dataset")
	if err := os.MkdirAll(filepath.Join(sourceDir, "empty"), 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}
	original := make([]byte, 6<<20)
	if _, err := rand.New(rand.NewSource(7)).Read(original); err != nil {
		t.Fatalf("generate fixture: %v", err)
	}
	primaryPath := filepath.Join(sourceDir, "primary.bin")
	duplicatePath := filepath.Join(sourceDir, "duplicate.bin")
	if err := os.WriteFile(primaryPath, original, 0o640); err != nil {
		t.Fatalf("write primary: %v", err)
	}
	if err := os.WriteFile(duplicatePath, original, 0o640); err != nil {
		t.Fatalf("write duplicate: %v", err)
	}

	key := sha256.Sum256([]byte("repository-test-key"))
	store := NewRepositoryStore(key[:])
	provider := newMemoryRepositoryProvider()
	task := TaskSpec{
		ID:          12,
		Name:        "repository-test",
		Type:        "file",
		SourcePaths: []string{sourceDir},
		Compression: "zstd",
		Encrypt:     true,
		StartedAt:   time.Date(2026, 8, 6, 1, 2, 3, 0, time.UTC),
		TempDir:     tempDir,
	}

	firstPlan, err := store.BuildPlan(ctx, task, NopLogWriter{})
	if err != nil {
		t.Fatalf("build first plan: %v", err)
	}
	firstKey := store.SnapshotKey(task.ID, 1, task.StartedAt)
	firstResult, err := store.Upload(ctx, provider, firstPlan, firstKey)
	if closeErr := firstPlan.Close(); closeErr != nil {
		t.Fatalf("close first plan: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("upload first snapshot: %v", err)
	}
	if firstResult.NewChunks == 0 || firstResult.UniqueChunks == 0 {
		t.Fatalf("first upload did not create chunks: %+v", firstResult)
	}
	if firstPlan.UniqueSize >= firstPlan.LogicalSize {
		t.Fatalf("duplicate file was not deduplicated within plan: unique=%d logical=%d", firstPlan.UniqueSize, firstPlan.LogicalSize)
	}

	modified := make([]byte, 0, len(original)+4096)
	modified = append(modified, original[:2<<20]...)
	modified = append(modified, bytes.Repeat([]byte("changed!"), 512)...)
	modified = append(modified, original[2<<20:]...)
	if err := os.WriteFile(primaryPath, modified, 0o640); err != nil {
		t.Fatalf("modify primary: %v", err)
	}
	task.StartedAt = task.StartedAt.Add(time.Hour)
	secondPlan, err := store.BuildPlan(ctx, task, NopLogWriter{})
	if err != nil {
		t.Fatalf("build second plan: %v", err)
	}
	secondKey := store.SnapshotKey(task.ID, 2, task.StartedAt)
	secondResult, err := store.Upload(ctx, provider, secondPlan, secondKey)
	if closeErr := secondPlan.Close(); closeErr != nil {
		t.Fatalf("close second plan: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("upload second snapshot: %v", err)
	}
	if secondResult.ReusedBytes <= secondResult.LogicalSize/3 {
		t.Fatalf("second snapshot reused too little data: %+v", secondResult)
	}
	if secondResult.UploadedBytes >= secondResult.LogicalSize {
		t.Fatalf("incremental upload was not smaller than logical data: %+v", secondResult)
	}

	verify, err := store.Verify(ctx, provider, secondKey, secondResult.Checksum)
	if err != nil {
		t.Fatalf("verify repository: %v", err)
	}
	if verify.Chunks == 0 || verify.Bytes == 0 {
		t.Fatalf("empty verification result: %+v", verify)
	}

	restoreRoot := filepath.Join(tempDir, "restore")
	restoreTask := task
	restoreTask.RestoreTargetPath = restoreRoot
	if err := store.Restore(ctx, provider, secondKey, restoreTask, NopLogWriter{}); err != nil {
		t.Fatalf("restore snapshot: %v", err)
	}
	restored, err := os.ReadFile(filepath.Join(restoreRoot, filepath.Base(sourceDir), "primary.bin"))
	if err != nil {
		t.Fatalf("read restored primary: %v", err)
	}
	if !bytes.Equal(restored, modified) {
		t.Fatalf("restored primary differs from source")
	}
	if info, err := os.Stat(filepath.Join(restoreRoot, filepath.Base(sourceDir), "empty")); err != nil || !info.IsDir() {
		t.Fatalf("empty directory was not restored: info=%v err=%v", info, err)
	}

	if err := provider.Delete(ctx, firstKey); err != nil {
		t.Fatalf("delete first snapshot: %v", err)
	}
	if _, err := store.Prune(ctx, provider); err != nil {
		t.Fatalf("prune with live snapshot: %v", err)
	}
	if err := provider.Delete(ctx, secondKey); err != nil {
		t.Fatalf("delete second snapshot: %v", err)
	}
	pruned, err := store.Prune(ctx, provider)
	if err != nil {
		t.Fatalf("prune empty repository: %v", err)
	}
	if pruned.DeletedPacks == 0 || pruned.DeletedIndexes == 0 {
		t.Fatalf("prune did not reclaim repository data: %+v", pruned)
	}
	if objects, err := provider.List(ctx, repositoryPackPrefix); err != nil || len(objects) != 0 {
		t.Fatalf("packs remain after prune: objects=%v err=%v", objects, err)
	}
}

type memoryRepositoryProvider struct {
	mu      sync.RWMutex
	objects map[string][]byte
	times   map[string]time.Time
}

func newMemoryRepositoryProvider() *memoryRepositoryProvider {
	return &memoryRepositoryProvider{objects: make(map[string][]byte), times: make(map[string]time.Time)}
}

func (p *memoryRepositoryProvider) Type() storage.ProviderType           { return "memory" }
func (p *memoryRepositoryProvider) TestConnection(context.Context) error { return nil }

func (p *memoryRepositoryProvider) Upload(_ context.Context, key string, reader io.Reader, size int64, _ map[string]string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if int64(len(data)) != size {
		return fmt.Errorf("size mismatch for %s: %d != %d", key, len(data), size)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.objects[key] = append([]byte(nil), data...)
	p.times[key] = time.Now().UTC()
	return nil
}

func (p *memoryRepositoryProvider) Download(_ context.Context, key string) (io.ReadCloser, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	data, ok := p.objects[key]
	if !ok {
		return nil, fmt.Errorf("object %s not found", key)
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), data...))), nil
}

func (p *memoryRepositoryProvider) DownloadRange(_ context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	data, ok := p.objects[key]
	if !ok {
		return nil, fmt.Errorf("object %s not found", key)
	}
	if offset < 0 || length <= 0 || offset+length > int64(len(data)) {
		return nil, fmt.Errorf("invalid range %d:%d for %s", offset, length, key)
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), data[offset:offset+length]...))), nil
}

func (p *memoryRepositoryProvider) Delete(_ context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.objects[key]; !ok {
		return fmt.Errorf("object %s not found", key)
	}
	delete(p.objects, key)
	delete(p.times, key)
	return nil
}

func (p *memoryRepositoryProvider) List(_ context.Context, prefix string) ([]storage.ObjectInfo, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]storage.ObjectInfo, 0)
	for key, data := range p.objects {
		if strings.HasPrefix(key, prefix) {
			result = append(result, storage.ObjectInfo{Key: key, Size: int64(len(data)), UpdatedAt: p.times[key]})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}
