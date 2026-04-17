package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListLocalDir(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world!"), 0644)

	entries, err := listLocalDir(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// 目录排序靠前
	if !entries[0].IsDir || entries[0].Name != "sub" {
		t.Errorf("directories should sort first: %+v", entries)
	}
	// 文件大小正确
	var a *DirEntry
	for i := range entries {
		if entries[i].Name == "a.txt" {
			a = &entries[i]
			break
		}
	}
	if a == nil || a.Size != 5 {
		t.Errorf("file size: %+v", a)
	}
}

func TestSplitCommaOrNewline(t *testing.T) {
	cases := []struct {
		in  string
		out []string
	}{
		{"", nil},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a\nb\nc", []string{"a", "b", "c"}},
		{"a; b ,\nc\n", []string{"a", "b", "c"}},
	}
	for _, c := range cases {
		got := splitCommaOrNewline(c.in)
		if len(got) != len(c.out) {
			t.Errorf("%q: got %v want %v", c.in, got, c.out)
			continue
		}
		for i := range got {
			if got[i] != c.out[i] {
				t.Errorf("%q[%d]: %q vs %q", c.in, i, got[i], c.out[i])
			}
		}
	}
}
