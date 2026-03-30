package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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

// SHA256File 计算文件的 SHA-256 哈希值，返回十六进制字符串
func SHA256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("compute checksum: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// SHA256Reader 计算 reader 的 SHA-256 哈希值，返回十六进制字符串
func SHA256Reader(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", fmt.Errorf("compute checksum: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
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
