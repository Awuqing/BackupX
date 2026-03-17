package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func createTempArtifact(baseDir, taskName string, extension string) (string, string, error) {
	tempDir, err := os.MkdirTemp(baseDir, "backupx-run-*")
	if err != nil {
		return "", "", fmt.Errorf("create temp dir: %w", err)
	}
	base := sanitizeFileName(taskName)
	if base == "" {
		base = "backup"
	}
	fileName := fmt.Sprintf("%s_%s.%s", base, time.Now().UTC().Format("20060102T150405"), strings.TrimPrefix(extension, "."))
	return tempDir, filepath.Join(tempDir, fileName), nil
}

func sanitizeFileName(value string) string {
	builder := strings.Builder{}
	for _, char := range strings.TrimSpace(value) {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char + ('a' - 'A'))
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '-' || char == '_':
			builder.WriteRune(char)
		case char == ' ' || char == '.':
			builder.WriteRune('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}
