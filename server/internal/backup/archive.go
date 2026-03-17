package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func CreateTarGz(ctx context.Context, sourcePath string, excludePatterns []string, destinationPath string, logger LogWriter) (int64, error) {
	sourcePath = filepath.Clean(strings.TrimSpace(sourcePath))
	if sourcePath == "" {
		return 0, fmt.Errorf("source path is required")
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return 0, fmt.Errorf("create destination directory: %w", err)
	}
	file, err := os.Create(destinationPath)
	if err != nil {
		return 0, fmt.Errorf("create archive file: %w", err)
	}
	defer file.Close()
	gzipWriter, err := gzip.NewWriterLevel(file, gzip.DefaultCompression)
	if err != nil {
		return 0, fmt.Errorf("create gzip writer: %w", err)
	}
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	baseParent := filepath.Dir(sourcePath)
	walkErr := filepath.Walk(sourcePath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		rel, err := filepath.Rel(baseParent, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if shouldExcludeArchive(rel, excludePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			if logger != nil {
				logger.WriteLine(fmt.Sprintf("跳过排除路径：%s", rel))
			}
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("build tar header: %w", err)
		}
		header.Name = rel
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header: %w", err)
		}
		if info.IsDir() {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open source file: %w", err)
		}
		defer input.Close()
		if _, err := io.Copy(tarWriter, input); err != nil {
			return fmt.Errorf("write tar body: %w", err)
		}
		return nil
	})
	if walkErr != nil {
		return 0, walkErr
	}
	if err := tarWriter.Close(); err != nil {
		return 0, fmt.Errorf("close tar writer: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return 0, fmt.Errorf("close gzip writer: %w", err)
	}
	if err := file.Close(); err != nil {
		return 0, fmt.Errorf("close archive file: %w", err)
	}
	info, err := os.Stat(destinationPath)
	if err != nil {
		return 0, fmt.Errorf("stat archive file: %w", err)
	}
	return info.Size(), nil
}

func ExtractTarGz(ctx context.Context, archivePath string, destinationDir string, logger LogWriter) error {
	archivePath = filepath.Clean(archivePath)
	destinationDir = filepath.Clean(destinationDir)
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive file: %w", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip reader: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		targetPath, err := secureJoin(destinationDir, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create restore directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create restore parent directory: %w", err)
			}
			output, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("create restore file: %w", err)
			}
			if _, err := io.Copy(output, tarReader); err != nil {
				output.Close()
				return fmt.Errorf("write restore file: %w", err)
			}
			if err := output.Close(); err != nil {
				return fmt.Errorf("close restore file: %w", err)
			}
			if logger != nil {
				logger.WriteLine(fmt.Sprintf("已恢复文件：%s", targetPath))
			}
		}
	}
}

func shouldExcludeArchive(rel string, patterns []string) bool {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	base := filepath.Base(rel)
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}
		if matched, _ := filepath.Match(trimmed, rel); matched {
			return true
		}
		if matched, _ := filepath.Match(trimmed, base); matched {
			return true
		}
		if strings.Contains(rel, trimmed) {
			return true
		}
	}
	return false
}

func secureJoin(root string, relative string) (string, error) {
	root = filepath.Clean(root)
	target := filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	rootWithSep := root + string(filepath.Separator)
	if target != root && !strings.HasPrefix(target, rootWithSep) {
		return "", fmt.Errorf("archive entry escapes destination: %s", relative)
	}
	return target, nil
}
