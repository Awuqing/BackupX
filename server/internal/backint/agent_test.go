package backint

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backupx/server/internal/storage"
	storageRclone "backupx/server/internal/storage/rclone"
)

// newTestAgent 构造一个使用本地磁盘后端的 Agent，便于集成测试。
func newTestAgent(t *testing.T, compress bool) (*Agent, string) {
	t.Helper()
	dir := t.TempDir()
	storageDir := filepath.Join(dir, "storage")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		t.Fatal(err)
	}

	registry := storage.NewRegistry(storageRclone.NewLocalDiskFactory())
	provider, err := registry.Create(context.Background(), "local_disk", map[string]any{
		"basePath": storageDir,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	cat, err := OpenCatalog(filepath.Join(dir, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	agent := &Agent{
		cfg:      &Config{StorageType: "local_disk", KeyPrefix: "backint", Compress: compress, CatalogDB: filepath.Join(dir, "catalog.db")},
		provider: provider,
		catalog:  cat,
	}
	t.Cleanup(func() { _ = agent.Close() })
	return agent, dir
}

func TestAgent_BackupAndRestore_File(t *testing.T) {
	agent, dir := newTestAgent(t, false)
	ctx := context.Background()

	// 准备源文件
	src := filepath.Join(dir, "src.bak")
	content := []byte("hello backint world")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	// BACKUP
	inPath := filepath.Join(dir, "backup.in")
	outPath := filepath.Join(dir, "backup.out")
	if err := os.WriteFile(inPath, []byte(src+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := agent.Run(ctx, FunctionBackup, inPath, outPath); err != nil {
		t.Fatalf("backup: %v", err)
	}
	out, _ := os.ReadFile(outPath)
	if !bytes.HasPrefix(out, []byte("#SAVED ")) {
		t.Fatalf("expected #SAVED, got: %s", out)
	}
	// 提取 EBID：#SAVED <ebid> "<path>"
	parts := strings.Fields(string(out))
	if len(parts) < 3 {
		t.Fatalf("malformed output: %s", out)
	}
	ebid := parts[1]

	// RESTORE
	restoreDst := filepath.Join(dir, "restored.bak")
	inPath2 := filepath.Join(dir, "restore.in")
	outPath2 := filepath.Join(dir, "restore.out")
	if err := os.WriteFile(inPath2, []byte(ebid+" \""+restoreDst+"\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := agent.Run(ctx, FunctionRestore, inPath2, outPath2); err != nil {
		t.Fatalf("restore: %v", err)
	}
	got, err := os.ReadFile(restoreDst)
	if err != nil {
		t.Fatalf("read restored: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("restored content mismatch: %q vs %q", got, content)
	}
}

func TestAgent_BackupWithCompression(t *testing.T) {
	agent, dir := newTestAgent(t, true)
	ctx := context.Background()

	src := filepath.Join(dir, "src.bak")
	content := bytes.Repeat([]byte("ABCDEFGH"), 1024)
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	inPath := filepath.Join(dir, "backup.in")
	outPath := filepath.Join(dir, "backup.out")
	_ = os.WriteFile(inPath, []byte(src+"\n"), 0644)
	if err := agent.Run(ctx, FunctionBackup, inPath, outPath); err != nil {
		t.Fatalf("backup: %v", err)
	}
	parts := strings.Fields(string(mustRead(t, outPath)))
	ebid := parts[1]

	// 验证 catalog 记录的对象键以 .gz 结尾
	entry, _ := agent.catalog.Get(ebid)
	if entry == nil || !strings.HasSuffix(entry.ObjectKey, ".gz") {
		t.Fatalf("expected .gz suffix: %+v", entry)
	}

	// RESTORE 应能解压回原始内容
	dst := filepath.Join(dir, "restored.bak")
	in2 := filepath.Join(dir, "restore.in")
	out2 := filepath.Join(dir, "restore.out")
	_ = os.WriteFile(in2, []byte(ebid+" \""+dst+"\"\n"), 0644)
	if err := agent.Run(ctx, FunctionRestore, in2, out2); err != nil {
		t.Fatalf("restore: %v", err)
	}
	got := mustRead(t, dst)
	if !bytes.Equal(got, content) {
		t.Errorf("decompressed content mismatch (len=%d vs %d)", len(got), len(content))
	}
}

func TestAgent_Inquire(t *testing.T) {
	agent, dir := newTestAgent(t, false)
	ctx := context.Background()

	// 注入两条目录记录
	_ = agent.catalog.Put(CatalogEntry{EBID: "bid-a", ObjectKey: "k/a"})
	_ = agent.catalog.Put(CatalogEntry{EBID: "bid-b", ObjectKey: "k/b"})

	// INQUIRE #NULL 应列出全部
	in := filepath.Join(dir, "inq.in")
	out := filepath.Join(dir, "inq.out")
	_ = os.WriteFile(in, []byte("#NULL\n"), 0644)
	if err := agent.Run(ctx, FunctionInquire, in, out); err != nil {
		t.Fatalf("inquire: %v", err)
	}
	text := string(mustRead(t, out))
	if !strings.Contains(text, "bid-a") || !strings.Contains(text, "bid-b") {
		t.Errorf("expected both ebids, got: %s", text)
	}

	// INQUIRE 不存在的 ebid → #NOTFOUND
	_ = os.WriteFile(in, []byte("bid-missing\n"), 0644)
	if err := agent.Run(ctx, FunctionInquire, in, out); err != nil {
		t.Fatalf("inquire missing: %v", err)
	}
	text = string(mustRead(t, out))
	if !strings.Contains(text, "#NOTFOUND") {
		t.Errorf("expected #NOTFOUND, got: %s", text)
	}
}

func TestAgent_Delete(t *testing.T) {
	agent, dir := newTestAgent(t, false)
	ctx := context.Background()

	// 先做一次 BACKUP
	src := filepath.Join(dir, "src.bak")
	_ = os.WriteFile(src, []byte("data"), 0644)
	inPath := filepath.Join(dir, "b.in")
	outPath := filepath.Join(dir, "b.out")
	_ = os.WriteFile(inPath, []byte(src+"\n"), 0644)
	if err := agent.Run(ctx, FunctionBackup, inPath, outPath); err != nil {
		t.Fatal(err)
	}
	ebid := strings.Fields(string(mustRead(t, outPath)))[1]

	// DELETE
	delIn := filepath.Join(dir, "d.in")
	delOut := filepath.Join(dir, "d.out")
	_ = os.WriteFile(delIn, []byte(ebid+"\n"), 0644)
	if err := agent.Run(ctx, FunctionDelete, delIn, delOut); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !strings.Contains(string(mustRead(t, delOut)), "#DELETED") {
		t.Errorf("expected #DELETED, got: %s", mustRead(t, delOut))
	}
	// catalog 条目应已删除
	if entry, _ := agent.catalog.Get(ebid); entry != nil {
		t.Errorf("catalog entry should be removed, got: %+v", entry)
	}
}

func TestAgent_RestoreUnknownEBID(t *testing.T) {
	agent, dir := newTestAgent(t, false)
	ctx := context.Background()

	in := filepath.Join(dir, "r.in")
	out := filepath.Join(dir, "r.out")
	_ = os.WriteFile(in, []byte("bid-unknown \""+filepath.Join(dir, "dst")+"\"\n"), 0644)
	if err := agent.Run(ctx, FunctionRestore, in, out); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(string(mustRead(t, out)), "#ERROR") {
		t.Errorf("expected #ERROR for unknown ebid, got: %s", mustRead(t, out))
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
