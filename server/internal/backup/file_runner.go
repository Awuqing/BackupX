package backup

import (
	"archive/tar"
	"context"
	"encoding/json"
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
	// 解析源路径列表：优先 SourcePaths，回退 SourcePath
	sourcePaths := task.SourcePaths
	if len(sourcePaths) == 0 && strings.TrimSpace(task.SourcePath) != "" {
		sourcePaths = []string{task.SourcePath}
	}
	if len(sourcePaths) == 0 {
		return nil, fmt.Errorf("source path is required")
	}

	// 验证所有路径存在
	for _, sp := range sourcePaths {
		cleaned := filepath.Clean(strings.TrimSpace(sp))
		if _, err := os.Stat(cleaned); err != nil {
			return nil, fmt.Errorf("stat source path %s: %w", cleaned, err)
		}
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

	excludes := normalizeExcludePatterns(task.ExcludePatterns)

	// 差异备份：基于上次全量清单仅打包新增/变更条目并记录删除；
	// 全量备份：记录完整清单（manifest）供后续差异比对。
	differential := task.Differential && len(task.BaseManifest.Entries) > 0
	baseIndex := map[string]ManifestEntry{}
	seen := map[string]struct{}{}
	var manifest *Manifest
	if differential {
		baseIndex = task.BaseManifest.index()
		writer.WriteLine(fmt.Sprintf("差异备份模式：基线含 %d 个条目", len(baseIndex)))
	} else {
		manifest = &Manifest{Entries: make([]ManifestEntry, 0)}
	}

	totalFileCount := 0
	totalDirCount := 0

	for i, sp := range sourcePaths {
		sourcePath := filepath.Clean(strings.TrimSpace(sp))
		info, err := os.Stat(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("stat source path: %w", err)
		}

		baseParent := filepath.Dir(sourcePath)
		writer.WriteLine(fmt.Sprintf("开始打包源路径 [%d/%d]: %s", i+1, len(sourcePaths), sourcePath))
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

			entry := entryFromInfo(archiveName, currentInfo)
			if differential {
				seen[entry.Path] = struct{}{}
				if !changedSince(baseIndex, entry) {
					return nil // 自全量以来未变更，跳过
				}
			} else {
				manifest.Entries = append(manifest.Entries, entry)
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
			return nil, fmt.Errorf("walk source path %s: %w", sourcePath, walkErr)
		}
		if info.IsDir() {
			writer.WriteLine(fmt.Sprintf("源路径 [%d/%d] 打包完成（%d 个目录，%d 个文件）", i+1, len(sourcePaths), dirCount, fileCount))
		} else {
			writer.WriteLine(fmt.Sprintf("源路径 [%d/%d] 文件打包完成", i+1, len(sourcePaths)))
		}
		totalFileCount += fileCount
		totalDirCount += dirCount
	}

	if differential {
		deletions := deletedPaths(baseIndex, seen)
		if err := writeDeletionsEntry(tw, deletions); err != nil {
			return nil, err
		}
		writer.WriteLine(fmt.Sprintf("差异备份完成（%d 个目录、%d 个文件变更，删除 %d 项）", totalDirCount, totalFileCount, len(deletions)))
	} else if len(sourcePaths) > 1 {
		writer.WriteLine(fmt.Sprintf("全部源路径打包完成（共 %d 个目录，%d 个文件）", totalDirCount, totalFileCount))
	}
	return &RunResult{ArtifactPath: artifactPath, FileName: filepath.Base(artifactPath), TempDir: tempDir, Manifest: manifest}, nil
}

func (r *FileRunner) Restore(_ context.Context, task TaskSpec, artifactPath string, writer LogWriter) error {
	artifactFile, err := os.Open(artifactPath)
	if err != nil {
		return fmt.Errorf("open tar artifact: %w", err)
	}
	defer artifactFile.Close()
	// 恢复目标：优先取 SourcePaths 的第一个路径的父目录，回退 SourcePath
	restoreSource := task.SourcePath
	if len(task.SourcePaths) > 0 {
		restoreSource = task.SourcePaths[0]
	}
	targetParent := filepath.Dir(filepath.Clean(strings.TrimSpace(restoreSource)))
	if err := os.MkdirAll(targetParent, 0o755); err != nil {
		return fmt.Errorf("create restore parent: %w", err)
	}
	var pendingDeletions []string
	tr := tar.NewReader(artifactFile)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		// 差异归档的删除清单不落地，留待提取完成后统一应用（避免被同批新增条目误删）。
		if header.Name == deletionsEntryName {
			data, readErr := io.ReadAll(tr)
			if readErr != nil {
				return fmt.Errorf("read deletions entry: %w", readErr)
			}
			if jsonErr := json.Unmarshal(data, &pendingDeletions); jsonErr != nil {
				return fmt.Errorf("parse deletions entry: %w", jsonErr)
			}
			continue
		}
		cleanName := path.Clean(strings.TrimSpace(header.Name))
		if cleanName == "." || cleanName == "" {
			continue
		}
		targetPath, ok := resolveWithinParent(targetParent, cleanName)
		if !ok {
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
	if err := applyDeletions(targetParent, pendingDeletions, writer); err != nil {
		return err
	}
	writer.WriteLine("文件恢复完成")
	return nil
}

// resolveWithinParent 将归档相对名安全解析为 targetParent 下的绝对路径；
// 越界（路径穿越）时返回 ok=false。提取与删除共用此校验，杜绝逃逸。
func resolveWithinParent(targetParent, name string) (string, bool) {
	targetPath := filepath.Clean(filepath.Join(targetParent, filepath.FromSlash(name)))
	cleanParent := filepath.Clean(targetParent)
	if targetPath == cleanParent {
		return targetPath, true
	}
	if !strings.HasPrefix(targetPath, cleanParent+string(filepath.Separator)) {
		return "", false
	}
	return targetPath, true
}

// writeDeletionsEntry 将差异备份的删除路径列表写入归档特殊条目。
func writeDeletionsEntry(tw *tar.Writer, deletions []string) error {
	payload, err := json.Marshal(deletions)
	if err != nil {
		return fmt.Errorf("marshal deletions: %w", err)
	}
	header := &tar.Header{Name: deletionsEntryName, Mode: 0o600, Size: int64(len(payload)), Typeflag: tar.TypeReg}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write deletions header: %w", err)
	}
	if _, err := tw.Write(payload); err != nil {
		return fmt.Errorf("write deletions body: %w", err)
	}
	return nil
}

// applyDeletions 在基线恢复之上删除差异归档记录的路径（仅差异备份恢复时存在）。
// 每个路径经 resolveWithinParent 校验，越界即报错；目标不存在视为已删除。
func applyDeletions(targetParent string, deletions []string, writer LogWriter) error {
	for _, name := range deletions {
		clean := path.Clean(strings.TrimSpace(name))
		if clean == "." || clean == "" {
			continue
		}
		targetPath, ok := resolveWithinParent(targetParent, clean)
		if !ok {
			return fmt.Errorf("deletion entry escapes restore path")
		}
		if err := os.RemoveAll(targetPath); err != nil {
			return fmt.Errorf("apply deletion %s: %w", clean, err)
		}
	}
	if len(deletions) > 0 {
		writer.WriteLine(fmt.Sprintf("已应用差异删除 %d 项", len(deletions)))
	}
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
