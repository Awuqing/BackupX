package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// deletionsEntryName 是差异备份归档中记录「自全量以来被删除路径」的特殊条目名。
// 恢复时该条目不落地为文件，而是用于在基线之上删除对应路径。
const deletionsEntryName = ".backupx/deletions.json"

// ManifestEntry 记录一次全量备份中单个归档条目（文件或目录）的指纹，
// 供差异备份比对「自全量以来的变化」。Path 为归档内相对名（slash 分隔，
// 与 tar header.Name 一致）。字段使用短键以压缩清单体积。
type ManifestEntry struct {
	Path      string `json:"p"`
	Size      int64  `json:"s"`
	ModTimeNs int64  `json:"m"`
	Mode      uint32 `json:"o"`
	IsDir     bool   `json:"d,omitempty"`
}

// Manifest 是一次全量备份的完整条目清单（文件与目录）。
type Manifest struct {
	Entries []ManifestEntry `json:"entries"`
}

// EncodeManifest 将清单序列化为紧凑 JSON。
func EncodeManifest(m Manifest) ([]byte, error) {
	return json.Marshal(m)
}

// DecodeManifest 反序列化清单；空输入返回空清单（视为「无基线」）。
func DecodeManifest(data []byte) (Manifest, error) {
	m := Manifest{}
	if len(data) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// index 构建 path -> entry 映射，便于差异比对 O(1) 查找。
func (m Manifest) index() map[string]ManifestEntry {
	idx := make(map[string]ManifestEntry, len(m.Entries))
	for _, e := range m.Entries {
		idx[e.Path] = e
	}
	return idx
}

// entryFromInfo 由归档名与文件信息构造指纹条目。
func entryFromInfo(archiveName string, info os.FileInfo) ManifestEntry {
	return ManifestEntry{
		Path:      filepath.ToSlash(archiveName),
		Size:      info.Size(),
		ModTimeNs: info.ModTime().UnixNano(),
		Mode:      uint32(info.Mode().Perm()),
		IsDir:     info.IsDir(),
	}
}

// changedSince 判断当前条目相对基线是否为「新增或变更」（即应纳入差异归档）。
//   - 不在基线中 → 新增，纳入；
//   - 已存在的目录 → 不携带数据，跳过（其下变更文件会各自判定）；
//   - 文件大小或 mtime 变化 → 变更，纳入（rsync 风格启发式）。
func changedSince(base map[string]ManifestEntry, cur ManifestEntry) bool {
	prev, ok := base[cur.Path]
	if !ok {
		return true
	}
	if cur.IsDir {
		return false
	}
	return prev.Size != cur.Size || prev.ModTimeNs != cur.ModTimeNs
}

// deletedPaths 返回基线中存在、但本次遍历未出现的路径（被删除的条目），按路径升序。
func deletedPaths(base map[string]ManifestEntry, seen map[string]struct{}) []string {
	deleted := make([]string, 0)
	for p := range base {
		if _, ok := seen[p]; !ok {
			deleted = append(deleted, p)
		}
	}
	sort.Strings(deleted)
	return deleted
}
