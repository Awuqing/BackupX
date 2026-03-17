package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var fileNameCleaner = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func EnsureTempRoot() (string, error) {
	root := filepath.Join(os.TempDir(), "backupx")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create backup temp root: %w", err)
	}
	return root, nil
}

func CreateTaskTempDir(taskName string, startedAt time.Time) (string, error) {
	root, err := EnsureTempRoot()
	if err != nil {
		return "", err
	}
	name := sanitizeTaskName(taskName)
	if name == "" {
		name = "backup"
	}
	path := filepath.Join(root, fmt.Sprintf("%s_%s", name, startedAt.UTC().Format("20060102_150405")))
	if err := os.MkdirAll(path, 0o755); err != nil {
		return "", fmt.Errorf("create task temp dir: %w", err)
	}
	return path, nil
}

func BuildArtifactName(taskName string, startedAt time.Time, extension string) string {
	name := sanitizeTaskName(taskName)
	if name == "" {
		name = "backup"
	}
	ext := strings.TrimSpace(extension)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("%s_%s%s", name, startedAt.UTC().Format("20060102_150405"), ext)
}

func BuildStorageKey(backupType string, startedAt time.Time, fileName string) string {
	typeName := strings.TrimSpace(strings.ToLower(backupType))
	if typeName == "" {
		typeName = "file"
	}
	return filepath.ToSlash(filepath.Join("BackupX", typeName, startedAt.UTC().Format("060102"), fileName))
}

func sanitizeTaskName(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	trimmed = strings.ReplaceAll(trimmed, " ", "-")
	trimmed = fileNameCleaner.ReplaceAllString(trimmed, "-")
	trimmed = strings.Trim(trimmed, "-._")
	return trimmed
}
