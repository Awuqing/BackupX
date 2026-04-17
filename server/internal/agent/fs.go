package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// DirEntry Agent 返回给 Master 的目录项。
type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// listLocalDir 列出 Agent 所在机器的指定路径。
func listLocalDir(path string) ([]DirEntry, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "" {
		cleaned = "/"
	}
	entries, err := os.ReadDir(cleaned)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	result := make([]DirEntry, 0, len(entries))
	for _, entry := range entries {
		info, _ := entry.Info()
		size := int64(0)
		if info != nil && !entry.IsDir() {
			size = info.Size()
		}
		result = append(result, DirEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(cleaned, entry.Name()),
			IsDir: entry.IsDir(),
			Size:  size,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}
