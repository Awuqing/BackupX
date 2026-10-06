package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArtifactNamesPreserveUnicodeAndRejectUnsafeCharacters(t *testing.T) {
	startedAt := time.Date(2026, 10, 6, 1, 2, 3, 0, time.UTC)
	cases := []struct {
		name   string
		legacy string
		shared string
	}{
		{"中文备份", "中文备份", "中文备份"},
		{"  MySQL 数据库  ", "mysql_数据库", "mysql-数据库"},
		{"../目录\\备份:\x00?*", "目录备份", "目录-备份"},
		{"... /\\:*?", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeFileName(tc.name); got != tc.legacy {
				t.Fatalf("sanitizeFileName(%q) = %q, want %q", tc.name, got, tc.legacy)
			}
			if got := sanitizeTaskName(tc.name); got != tc.shared {
				t.Fatalf("sanitizeTaskName(%q) = %q, want %q", tc.name, got, tc.shared)
			}
			prefix := tc.shared
			if prefix == "" {
				prefix = "backup"
			}
			if got := BuildArtifactName(tc.name, startedAt, "sql"); got != prefix+"_20261006_010203.sql" {
				t.Fatalf("unexpected artifact name: %q", got)
			}
			tempDir, artifactPath, err := createTempArtifact(t.TempDir(), tc.name, "tar")
			if err != nil {
				t.Fatal(err)
			}
			prefix = tc.legacy
			if prefix == "" {
				prefix = "backup"
			}
			if filepath.Dir(artifactPath) != tempDir || !strings.HasPrefix(filepath.Base(artifactPath), prefix+"_") {
				t.Fatalf("unsafe or incorrect artifact path: %q", artifactPath)
			}
		})
	}
	t.Setenv("TMPDIR", t.TempDir())
	taskDir, err := CreateTaskTempDir("中文备份", startedAt)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(taskDir) != "中文备份_20261006_010203" {
		t.Fatalf("unexpected task directory: %q", taskDir)
	}
	if _, err := os.Stat(taskDir); err != nil {
		t.Fatal(err)
	}
}
