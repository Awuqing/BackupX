package compress

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func GzipFile(sourcePath string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open source file: %w", err)
	}
	defer source.Close()
	targetPath := sourcePath + ".gz"
	target, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("create gzip file: %w", err)
	}
	defer target.Close()
	writer := gzip.NewWriter(target)
	writer.Name = filepath.Base(sourcePath)
	if _, err := io.Copy(writer, source); err != nil {
		writer.Close()
		return "", fmt.Errorf("gzip source file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close gzip writer: %w", err)
	}
	return targetPath, nil
}

func GunzipFile(sourcePath string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open gzip file: %w", err)
	}
	defer source.Close()
	reader, err := gzip.NewReader(source)
	if err != nil {
		return "", fmt.Errorf("create gzip reader: %w", err)
	}
	defer reader.Close()
	targetPath := strings.TrimSuffix(sourcePath, ".gz")
	if targetPath == sourcePath {
		targetPath += ".out"
	}
	target, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("create target file: %w", err)
	}
	defer target.Close()
	if _, err := io.Copy(target, reader); err != nil {
		return "", fmt.Errorf("gunzip file: %w", err)
	}
	return targetPath, nil
}
