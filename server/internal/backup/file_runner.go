package backup

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type FileRunner struct{}

func NewFileRunner() *FileRunner {
	return &FileRunner{}
}

func (r *FileRunner) Type() string {
	return "file"
}

func (r *FileRunner) Run(_ context.Context, task TaskSpec, writer LogWriter) (*RunResult, error) {
	sourcePath := filepath.Clean(strings.TrimSpace(task.SourcePath))
	if sourcePath == "" {
		return nil, fmt.Errorf("source path is required")
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("stat source path: %w", err)
	}
	tempDir, artifactPath, err := createTempArtifact(task.TempDir, task.Name, "tar")
	if err != nil {
		return nil, err
	}
	artifactFile, err := os.Create(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("create tar artifact: %w", err)
	}
	defer artifactFile.Close()
	tw := tar.NewWriter(artifactFile)
	defer tw.Close()
	baseParent := filepath.Dir(sourcePath)
	excludes := normalizeExcludePatterns(task.ExcludePatterns)
	writer.WriteLine(fmt.Sprintf("开始打包文件备份：%s", sourcePath))
	fileCount := 0
	dirCount := 0
	walkErr := filepath.Walk(sourcePath, func(currentPath string, currentInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			writer.WriteLine(fmt.Sprintf("⚠ 无法访问 %s: %v", currentPath, walkErr))
			return nil
		}
		relPath, err := filepath.Rel(baseParent, currentPath)
		if err != nil {
			return err
		}
		archiveName := filepath.ToSlash(relPath)
		if shouldExcludeEntry(archiveName, currentInfo.IsDir(), excludes) {
			if currentInfo.IsDir() {
				writer.WriteLine(fmt.Sprintf("跳过排除目录 %s", archiveName))
				return filepath.SkipDir
			}
			return nil
		}
		if currentPath == sourcePath && currentInfo.IsDir() {
			return nil
		}

		if currentInfo.IsDir() {
			dirCount++
			writer.WriteLine(fmt.Sprintf("📁 进入目录 %s", archiveName))
		}

		header, err := tar.FileInfoHeader(currentInfo, "")
		if err != nil {
			return err
		}
		header.Name = archiveName
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if currentInfo.Mode().IsRegular() {
			file, err := os.Open(currentPath)
			if err != nil {
				return err
			}
			defer file.Close()
			if _, err := io.CopyN(tw, file, currentInfo.Size()); err != nil && err != io.EOF {
				return err
			}
			fileCount++
			if fileCount%100 == 0 {
				writer.WriteLine(fmt.Sprintf("已打包 %d 个文件...", fileCount))
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk source path: %w", walkErr)
	}
	if info.IsDir() {
		writer.WriteLine(fmt.Sprintf("目录打包完成（%d 个目录，%d 个文件）", dirCount, fileCount))
	} else {
		writer.WriteLine("文件打包完成")
	}
	return &RunResult{ArtifactPath: artifactPath, FileName: filepath.Base(artifactPath), TempDir: tempDir}, nil
}

func (r *FileRunner) Restore(_ context.Context, task TaskSpec, artifactPath string, writer LogWriter) error {
	artifactFile, err := os.Open(artifactPath)
	if err != nil {
		return fmt.Errorf("open tar artifact: %w", err)
	}
	defer artifactFile.Close()
	targetParent := filepath.Dir(filepath.Clean(strings.TrimSpace(task.SourcePath)))
	if err := os.MkdirAll(targetParent, 0o755); err != nil {
		return fmt.Errorf("create restore parent: %w", err)
	}
	tr := tar.NewReader(artifactFile)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		cleanName := path.Clean(strings.TrimSpace(header.Name))
		if cleanName == "." || cleanName == "" {
			continue
		}
		targetPath := filepath.Clean(filepath.Join(targetParent, filepath.FromSlash(cleanName)))
		parentWithSep := filepath.Clean(targetParent) + string(filepath.Separator)
		if targetPath != filepath.Clean(targetParent) && !strings.HasPrefix(targetPath, parentWithSep) {
			return fmt.Errorf("tar entry escapes restore path")
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create restore dir: %w", err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create restore parent dir: %w", err)
			}
			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create restore file: %w", err)
			}
			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return fmt.Errorf("write restore file: %w", err)
			}
			if err := file.Close(); err != nil {
				return fmt.Errorf("close restore file: %w", err)
			}
		}
	}
	writer.WriteLine("文件恢复完成")
	return nil
}

func normalizeExcludePatterns(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, filepath.ToSlash(trimmed))
		}
	}
	return result
}

func shouldExcludeEntry(relPath string, isDir bool, patterns []string) bool {
	relPath = filepath.ToSlash(relPath)
	base := path.Base(relPath)
	for _, pattern := range patterns {
		if matched, _ := path.Match(pattern, relPath); matched {
			return true
		}
		if matched, _ := path.Match(pattern, base); matched {
			return true
		}
		if isDir && strings.TrimSuffix(pattern, "/") == base {
			return true
		}
	}
	return false
}
