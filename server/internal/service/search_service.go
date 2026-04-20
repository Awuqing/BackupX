package service

import (
	"context"
	"strings"

	"backupx/server/internal/repository"
)

// SearchService 跨任务/存储目标/最近备份记录的全局搜索。
// 设计权衡：
//   - 只搜最近 100 条备份记录，避免全表扫描
//   - 所有 Name / Description / Tags / 文件名字段都做 Contains 匹配
//   - 返回结果按类型分组，前端分栏展示
type SearchService struct {
	tasks   repository.BackupTaskRepository
	records repository.BackupRecordRepository
	targets repository.StorageTargetRepository
	nodes   repository.NodeRepository
}

func NewSearchService(
	tasks repository.BackupTaskRepository,
	records repository.BackupRecordRepository,
	targets repository.StorageTargetRepository,
	nodes repository.NodeRepository,
) *SearchService {
	return &SearchService{tasks: tasks, records: records, targets: targets, nodes: nodes}
}

// SearchResultItem 统一结果项。
// URL 前端据此生成跳转链接，Highlight 显示匹配字段。
type SearchResultItem struct {
	Kind      string `json:"kind"` // task | record | storage | node
	ID        uint   `json:"id"`
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle,omitempty"`
	Highlight string `json:"highlight,omitempty"`
	URL       string `json:"url"`
}

// SearchResult 全局搜索总结果。
type SearchResult struct {
	Query      string             `json:"query"`
	Tasks      []SearchResultItem `json:"tasks"`
	Records    []SearchResultItem `json:"records"`
	Storage    []SearchResultItem `json:"storage"`
	Nodes      []SearchResultItem `json:"nodes"`
	TotalCount int                `json:"totalCount"`
}

// Search 执行全局搜索。空 query 返回空结果。
// 每类最多返回 10 条，避免页面过长。
func (s *SearchService) Search(ctx context.Context, query string) (*SearchResult, error) {
	q := strings.TrimSpace(query)
	result := &SearchResult{Query: q, Tasks: []SearchResultItem{}, Records: []SearchResultItem{}, Storage: []SearchResultItem{}, Nodes: []SearchResultItem{}}
	if q == "" {
		return result, nil
	}
	lowerQ := strings.ToLower(q)

	// 搜任务
	if s.tasks != nil {
		if items, err := s.tasks.List(ctx, repository.BackupTaskListOptions{}); err == nil {
			for _, item := range items {
				if !matchesAny(lowerQ, item.Name, item.Type, item.Tags, item.SourcePath, item.DBHost, item.DBName) {
					continue
				}
				hl := firstMatch(lowerQ, item.Name, item.Tags)
				result.Tasks = append(result.Tasks, SearchResultItem{
					Kind:      "task",
					ID:        item.ID,
					Title:     item.Name,
					Subtitle:  item.Type,
					Highlight: hl,
					URL:       "/backup/tasks",
				})
				if len(result.Tasks) >= 10 {
					break
				}
			}
		}
	}

	// 搜存储目标
	if s.targets != nil {
		if items, err := s.targets.List(ctx); err == nil {
			for _, item := range items {
				if !matchesAny(lowerQ, item.Name, item.Description, item.Type) {
					continue
				}
				hl := firstMatch(lowerQ, item.Name, item.Type)
				result.Storage = append(result.Storage, SearchResultItem{
					Kind:      "storage",
					ID:        item.ID,
					Title:     item.Name,
					Subtitle:  item.Type,
					Highlight: hl,
					URL:       "/storage-targets",
				})
				if len(result.Storage) >= 10 {
					break
				}
			}
		}
	}

	// 搜节点
	if s.nodes != nil {
		if items, err := s.nodes.List(ctx); err == nil {
			for _, item := range items {
				if !matchesAny(lowerQ, item.Name, item.Hostname, item.IPAddress) {
					continue
				}
				hl := firstMatch(lowerQ, item.Name, item.Hostname, item.IPAddress)
				result.Nodes = append(result.Nodes, SearchResultItem{
					Kind:      "node",
					ID:        item.ID,
					Title:     item.Name,
					Subtitle:  item.Hostname,
					Highlight: hl,
					URL:       "/nodes",
				})
				if len(result.Nodes) >= 10 {
					break
				}
			}
		}
	}

	// 搜最近 100 条备份记录（文件名）
	if s.records != nil {
		if items, err := s.records.ListRecent(ctx, 100); err == nil {
			for _, item := range items {
				if !matchesAny(lowerQ, item.FileName, item.StoragePath, item.Task.Name) {
					continue
				}
				hl := firstMatch(lowerQ, item.FileName, item.StoragePath)
				result.Records = append(result.Records, SearchResultItem{
					Kind:      "record",
					ID:        item.ID,
					Title:     item.FileName,
					Subtitle:  item.Task.Name,
					Highlight: hl,
					URL:       "/backup/records?recordId=" + itoaUint(item.ID),
				})
				if len(result.Records) >= 10 {
					break
				}
			}
		}
	}

	result.TotalCount = len(result.Tasks) + len(result.Records) + len(result.Storage) + len(result.Nodes)
	return result, nil
}

// matchesAny 忽略大小写匹配任一字段。
func matchesAny(lowerQ string, fields ...string) bool {
	for _, f := range fields {
		if f == "" {
			continue
		}
		if strings.Contains(strings.ToLower(f), lowerQ) {
			return true
		}
	}
	return false
}

// firstMatch 返回第一个匹配的字段值（用于 Highlight）。
func firstMatch(lowerQ string, fields ...string) string {
	for _, f := range fields {
		if f == "" {
			continue
		}
		if strings.Contains(strings.ToLower(f), lowerQ) {
			return f
		}
	}
	return ""
}

func itoaUint(v uint) string {
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 12)
	n := v
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
