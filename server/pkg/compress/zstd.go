package compress

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// ZstdFile 将文件压缩为 .zst（zstd），返回压缩产物路径。
// 相比 gzip，zstd 在相近 CPU 开销下提供更高压缩率与显著更快的解压速度。
func ZstdFile(sourcePath string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open source file: %w", err)
	}
	defer source.Close()
	targetPath := sourcePath + ".zst"
	target, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("create zstd file: %w", err)
	}
	defer target.Close()
	writer, err := zstd.NewWriter(target)
	if err != nil {
		return "", fmt.Errorf("create zstd writer: %w", err)
	}
	if _, err := io.Copy(writer, source); err != nil {
		_ = writer.Close()
		return "", fmt.Errorf("zstd source file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close zstd writer: %w", err)
	}
	return targetPath, nil
}

// UnzstdFile 解压 .zst 文件，返回解压产物路径。
func UnzstdFile(sourcePath string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open zstd file: %w", err)
	}
	defer source.Close()
	reader, err := zstd.NewReader(source)
	if err != nil {
		return "", fmt.Errorf("create zstd reader: %w", err)
	}
	defer reader.Close()
	targetPath := strings.TrimSuffix(sourcePath, ".zst")
	if targetPath == sourcePath {
		targetPath += ".out"
	}
	target, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("create target file: %w", err)
	}
	defer target.Close()
	if _, err := io.Copy(target, reader); err != nil {
		return "", fmt.Errorf("unzstd file: %w", err)
	}
	return targetPath, nil
}
