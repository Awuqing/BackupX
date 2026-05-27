package backup

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func diffWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func diffAssertContent(t *testing.T, p, want string) {
	t.Helper()
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	if string(got) != want {
		t.Fatalf("%s content = %q, want %q", p, string(got), want)
	}
}

func diffAssertAbsent(t *testing.T, p string) {
	t.Helper()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, stat err=%v", p, err)
	}
}

func diffArchiveNames(t *testing.T, artifactPath string) map[string]bool {
	t.Helper()
	f, err := os.Open(artifactPath)
	if err != nil {
		t.Fatalf("open artifact: %v", err)
	}
	defer f.Close()
	names := map[string]bool{}
	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		names[h.Name] = true
	}
	return names
}

// TestFileRunnerDifferentialRoundTrip 验证差异备份的端到端正确性：
// 全量 → 修改源（变更/删除/新增）→ 差异 → 链式恢复（全量+差异）→ 结果与修改后源一致。
func TestFileRunnerDifferentialRoundTrip(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "src")
	diffWrite(t, filepath.Join(src, "a.txt"), "alpha")
	diffWrite(t, filepath.Join(src, "b.txt"), "bravo")
	diffWrite(t, filepath.Join(src, "sub", "c.txt"), "charlie")

	runner := NewFileRunner()

	full, err := runner.Run(context.Background(), TaskSpec{Name: "diff", Type: "file", SourcePath: src, TempDir: t.TempDir()}, NopLogWriter{})
	if err != nil {
		t.Fatalf("full Run: %v", err)
	}
	if full.Manifest == nil || len(full.Manifest.Entries) == 0 {
		t.Fatalf("full backup must produce a manifest, got %#v", full.Manifest)
	}

	// 变更 a.txt（内容变长 → size 差异必被检出）、删除 b.txt、新增 d.txt；sub/c.txt 不变
	diffWrite(t, filepath.Join(src, "a.txt"), "ALPHA-modified-and-longer")
	if err := os.Remove(filepath.Join(src, "b.txt")); err != nil {
		t.Fatalf("remove b.txt: %v", err)
	}
	diffWrite(t, filepath.Join(src, "d.txt"), "delta")

	diff, err := runner.Run(context.Background(), TaskSpec{Name: "diff", Type: "file", SourcePath: src, TempDir: t.TempDir(), Differential: true, BaseManifest: *full.Manifest}, NopLogWriter{})
	if err != nil {
		t.Fatalf("differential Run: %v", err)
	}
	if diff.Manifest != nil {
		t.Fatalf("differential backup must not produce a manifest")
	}

	// 差异归档应包含变更/新增条目与删除清单，但不含未变更的 sub/c.txt
	names := diffArchiveNames(t, diff.ArtifactPath)
	if !names["src/a.txt"] || !names["src/d.txt"] {
		t.Fatalf("differential archive missing changed/new entries: %v", names)
	}
	if names["src/sub/c.txt"] {
		t.Fatalf("differential archive should not contain unchanged file sub/c.txt")
	}
	if !names[deletionsEntryName] {
		t.Fatalf("differential archive missing deletions entry: %v", names)
	}

	// 链式恢复到全新目标
	restoreRoot := t.TempDir()
	restoreSrc := filepath.Join(restoreRoot, "src")
	restoreTask := TaskSpec{Name: "diff", Type: "file", SourcePath: restoreSrc}
	if err := runner.Restore(context.Background(), restoreTask, full.ArtifactPath, NopLogWriter{}); err != nil {
		t.Fatalf("restore full: %v", err)
	}
	if err := runner.Restore(context.Background(), restoreTask, diff.ArtifactPath, NopLogWriter{}); err != nil {
		t.Fatalf("restore differential: %v", err)
	}

	diffAssertContent(t, filepath.Join(restoreSrc, "a.txt"), "ALPHA-modified-and-longer")
	diffAssertContent(t, filepath.Join(restoreSrc, "sub", "c.txt"), "charlie")
	diffAssertContent(t, filepath.Join(restoreSrc, "d.txt"), "delta")
	diffAssertAbsent(t, filepath.Join(restoreSrc, "b.txt"))
}

func TestPathSelected(t *testing.T) {
	sel := []string{"src/a.txt", "src/sub"}
	cases := map[string]bool{
		"src/a.txt":     true,
		"src/sub":       true,
		"src/sub/c.txt": true,  // 选中目录下的子项
		"src/b.txt":     false, // 未选中文件
		"src/subother":  false, // 前缀相近但非子项，不应误判
	}
	for name, want := range cases {
		if got := pathSelected(name, sel); got != want {
			t.Errorf("pathSelected(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestFileRunnerSelectiveRestore 验证按需恢复：仅选中的文件与目录被还原，未选中的文件不出现。
func TestFileRunnerSelectiveRestore(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "src")
	diffWrite(t, filepath.Join(src, "a.txt"), "alpha")
	diffWrite(t, filepath.Join(src, "b.txt"), "bravo")
	diffWrite(t, filepath.Join(src, "sub", "c.txt"), "charlie")

	runner := NewFileRunner()
	full, err := runner.Run(context.Background(), TaskSpec{Name: "sel", Type: "file", SourcePath: src, TempDir: t.TempDir()}, NopLogWriter{})
	if err != nil {
		t.Fatalf("full Run: %v", err)
	}

	restoreRoot := t.TempDir()
	restoreSrc := filepath.Join(restoreRoot, "src")
	task := TaskSpec{Name: "sel", Type: "file", SourcePath: restoreSrc, SelectedPaths: []string{"src/a.txt", "src/sub"}}
	if err := runner.Restore(context.Background(), task, full.ArtifactPath, NopLogWriter{}); err != nil {
		t.Fatalf("selective Restore: %v", err)
	}
	diffAssertContent(t, filepath.Join(restoreSrc, "a.txt"), "alpha")
	diffAssertContent(t, filepath.Join(restoreSrc, "sub", "c.txt"), "charlie") // 选中目录 → 子项一并恢复
	diffAssertAbsent(t, filepath.Join(restoreSrc, "b.txt"))                     // 未选中 → 不恢复
}

// TestFileRunnerDifferentialWithoutBaseIsFull 验证无基线时差异请求回退为全量（产出清单、含全部文件）。
func TestFileRunnerDifferentialWithoutBaseIsFull(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	diffWrite(t, filepath.Join(src, "a.txt"), "alpha")

	runner := NewFileRunner()
	res, err := runner.Run(context.Background(), TaskSpec{Name: "diff", Type: "file", SourcePath: src, TempDir: t.TempDir(), Differential: true}, NopLogWriter{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Manifest == nil {
		t.Fatalf("differential without base must fall back to full and produce a manifest")
	}
	if names := diffArchiveNames(t, res.ArtifactPath); !names["src/a.txt"] || names[deletionsEntryName] {
		t.Fatalf("fallback-full archive unexpected: %v", names)
	}
}
