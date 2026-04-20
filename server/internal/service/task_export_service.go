package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// TaskExportService 管理备份任务的 JSON 导入 / 导出。
// 用途：
//   1. 集群迁移（旧 Master → 新 Master 的任务配置搬迁）
//   2. 灾备恢复（任务配置本地文件化，Master 宕机后重建）
//   3. 配置审计（版本化 Git 管理 JSON 快照）
//
// 出于安全考虑，导出/导入不包含任何敏感字段：
//   - 数据库密码（DBPasswordCiphertext）：跳过，导入后需人工填补
//   - 存储目标具体配置：仅按 name 匹配现有目标，不搬运密钥
//   - Node 绑定：按 name 匹配现有节点，不存在时退化为 NodeID=0（本机）
type TaskExportService struct {
	tasks    *BackupTaskService
	taskRepo repository.BackupTaskRepository
	targets  repository.StorageTargetRepository
	nodes    repository.NodeRepository
}

func NewTaskExportService(
	tasks *BackupTaskService,
	taskRepo repository.BackupTaskRepository,
	targets repository.StorageTargetRepository,
	nodes repository.NodeRepository,
) *TaskExportService {
	return &TaskExportService{tasks: tasks, taskRepo: taskRepo, targets: targets, nodes: nodes}
}

// ExportedTask 导出格式：按名称引用存储/节点，不含敏感数据。
type ExportedTask struct {
	Name               string         `json:"name"`
	Type               string         `json:"type"`
	Enabled            bool           `json:"enabled"`
	CronExpr           string         `json:"cronExpr,omitempty"`
	SourcePath         string         `json:"sourcePath,omitempty"`
	SourcePaths        []string       `json:"sourcePaths,omitempty"`
	ExcludePatterns    []string       `json:"excludePatterns,omitempty"`
	DBHost             string         `json:"dbHost,omitempty"`
	DBPort             int            `json:"dbPort,omitempty"`
	DBUser             string         `json:"dbUser,omitempty"`
	DBName             string         `json:"dbName,omitempty"`
	DBPath             string         `json:"dbPath,omitempty"`
	ExtraConfig        map[string]any `json:"extraConfig,omitempty"`
	// 按名称引用：导入时按名称查找对应 ID
	StorageTargetNames    []string `json:"storageTargetNames"`
	ReplicationTargetNames []string `json:"replicationTargetNames,omitempty"`
	NodeName              string   `json:"nodeName,omitempty"`
	DependsOnTaskNames    []string `json:"dependsOnTaskNames,omitempty"`
	Tags                  string   `json:"tags,omitempty"`
	Compression           string   `json:"compression,omitempty"`
	Encrypt               bool     `json:"encrypt,omitempty"`
	RetentionDays         int      `json:"retentionDays,omitempty"`
	MaxBackups            int      `json:"maxBackups,omitempty"`
	VerifyEnabled         bool     `json:"verifyEnabled,omitempty"`
	VerifyCronExpr        string   `json:"verifyCronExpr,omitempty"`
	VerifyMode            string   `json:"verifyMode,omitempty"`
	SLAHoursRPO           int      `json:"slaHoursRpo,omitempty"`
	AlertOnConsecutiveFails int    `json:"alertOnConsecutiveFails,omitempty"`
	MaintenanceWindows    string   `json:"maintenanceWindows,omitempty"`
}

// ExportPayload 导出整体结构，带元信息。
type ExportPayload struct {
	Version    string         `json:"version"`
	ExportedAt time.Time      `json:"exportedAt"`
	TaskCount  int            `json:"taskCount"`
	Tasks      []ExportedTask `json:"tasks"`
	Notice     string         `json:"notice"`
}

// ImportResult 导入单条结果，best-effort。
type ImportResult struct {
	Name    string `json:"name"`
	TaskID  uint   `json:"taskId,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
}

// Export 导出当前全部任务为 JSON。
// taskIDs 为空则导出全部；否则仅导出指定 ID。
func (s *TaskExportService) Export(ctx context.Context, taskIDs []uint) (*ExportPayload, error) {
	items, err := s.taskRepo.List(ctx, repository.BackupTaskListOptions{})
	if err != nil {
		return nil, apperror.Internal("TASK_EXPORT_LIST_FAILED", "无法获取任务列表", err)
	}
	targetNames := map[uint]string{}
	if all, err := s.targets.List(ctx); err == nil {
		for _, t := range all {
			targetNames[t.ID] = t.Name
		}
	}
	nodeNames := map[uint]string{}
	if all, err := s.nodes.List(ctx); err == nil {
		for _, n := range all {
			nodeNames[n.ID] = n.Name
		}
	}
	taskNames := map[uint]string{}
	for _, t := range items {
		taskNames[t.ID] = t.Name
	}
	idFilter := map[uint]bool{}
	for _, id := range taskIDs {
		idFilter[id] = true
	}
	exported := make([]ExportedTask, 0, len(items))
	for i := range items {
		item := items[i]
		if len(idFilter) > 0 && !idFilter[item.ID] {
			continue
		}
		et := s.toExported(&item, targetNames, nodeNames, taskNames)
		exported = append(exported, et)
	}
	return &ExportPayload{
		Version:    "v1",
		ExportedAt: time.Now().UTC(),
		TaskCount:  len(exported),
		Tasks:      exported,
		Notice:     "敏感字段（数据库密码、存储凭证）已排除，导入后需人工补全。",
	}, nil
}

// Import 批量导入任务。best-effort：单条失败不阻断。
// 冲突策略：任务名重复则跳过（不覆盖）。
func (s *TaskExportService) Import(ctx context.Context, payload ExportPayload) ([]ImportResult, error) {
	// 预加载所有命名 → ID 映射
	targetsByName := map[string]uint{}
	if all, err := s.targets.List(ctx); err == nil {
		for _, t := range all {
			targetsByName[t.Name] = t.ID
		}
	}
	nodesByName := map[string]uint{}
	if all, err := s.nodes.List(ctx); err == nil {
		for _, n := range all {
			nodesByName[n.Name] = n.ID
		}
	}
	tasksByName := map[string]uint{}
	existing, err := s.taskRepo.List(ctx, repository.BackupTaskListOptions{})
	if err != nil {
		return nil, apperror.Internal("TASK_IMPORT_LIST_FAILED", "无法读取当前任务列表", err)
	}
	for _, t := range existing {
		tasksByName[t.Name] = t.ID
	}
	results := make([]ImportResult, 0, len(payload.Tasks))
	// 两阶段：先创建所有任务（忽略 DependsOn），再更新依赖
	created := map[string]uint{}
	for _, t := range payload.Tasks {
		if t.Name == "" {
			continue
		}
		if _, dup := tasksByName[t.Name]; dup {
			results = append(results, ImportResult{Name: t.Name, Skipped: true, Success: true, Error: "已存在同名任务，跳过"})
			continue
		}
		input := s.toUpsertInput(t, targetsByName, nodesByName, nil)
		detail, err := s.tasks.Create(ctx, input)
		if err != nil {
			results = append(results, ImportResult{Name: t.Name, Success: false, Error: appErrorMessage(err)})
			continue
		}
		created[t.Name] = detail.ID
		tasksByName[t.Name] = detail.ID
		results = append(results, ImportResult{Name: t.Name, TaskID: detail.ID, Success: true})
	}
	// 第二阶段：依赖链接（上游任务名 → 新 ID）
	for i, t := range payload.Tasks {
		if len(t.DependsOnTaskNames) == 0 {
			continue
		}
		id, ok := created[t.Name]
		if !ok {
			continue
		}
		deps := []uint{}
		for _, name := range t.DependsOnTaskNames {
			if depID, ok := tasksByName[name]; ok && depID != id {
				deps = append(deps, depID)
			}
		}
		if len(deps) == 0 {
			continue
		}
		input := s.toUpsertInput(t, targetsByName, nodesByName, deps)
		if _, err := s.tasks.Update(ctx, id, input); err != nil {
			// 已创建但依赖更新失败：降级为 warning，不影响任务本体
			for idx := range results {
				if results[idx].Name == t.Name {
					results[idx].Error = fmt.Sprintf("任务已创建，但依赖更新失败: %s", appErrorMessage(err))
					break
				}
			}
			_ = i
		}
	}
	return results, nil
}

func (s *TaskExportService) toExported(item *model.BackupTask, targetNames, nodeNames, taskNames map[uint]string) ExportedTask {
	sourcePaths := []string{}
	if strings.TrimSpace(item.SourcePaths) != "" {
		_ = json.Unmarshal([]byte(item.SourcePaths), &sourcePaths)
	}
	excludes := []string{}
	if strings.TrimSpace(item.ExcludePatterns) != "" {
		_ = json.Unmarshal([]byte(item.ExcludePatterns), &excludes)
	}
	var extra map[string]any
	if strings.TrimSpace(item.ExtraConfig) != "" {
		_ = json.Unmarshal([]byte(item.ExtraConfig), &extra)
	}
	storageNames := namesFromIDs(collectTargetIDs(item), targetNames)
	replicationNames := namesFromIDs(parseUintCSV(item.ReplicationTargetIDs), targetNames)
	dependsOnNames := namesFromIDs(parseUintCSV(item.DependsOnTaskIDs), taskNames)
	nodeName := ""
	if item.NodeID > 0 {
		nodeName = nodeNames[item.NodeID]
	}
	return ExportedTask{
		Name:                   item.Name,
		Type:                   item.Type,
		Enabled:                item.Enabled,
		CronExpr:               item.CronExpr,
		SourcePath:             item.SourcePath,
		SourcePaths:            sourcePaths,
		ExcludePatterns:        excludes,
		DBHost:                 item.DBHost,
		DBPort:                 item.DBPort,
		DBUser:                 item.DBUser,
		DBName:                 item.DBName,
		DBPath:                 item.DBPath,
		ExtraConfig:            extra,
		StorageTargetNames:     storageNames,
		ReplicationTargetNames: replicationNames,
		NodeName:               nodeName,
		DependsOnTaskNames:     dependsOnNames,
		Tags:                   item.Tags,
		Compression:            item.Compression,
		Encrypt:                item.Encrypt,
		RetentionDays:          item.RetentionDays,
		MaxBackups:             item.MaxBackups,
		VerifyEnabled:          item.VerifyEnabled,
		VerifyCronExpr:         item.VerifyCronExpr,
		VerifyMode:             item.VerifyMode,
		SLAHoursRPO:            item.SLAHoursRPO,
		AlertOnConsecutiveFails: item.AlertOnConsecutiveFails,
		MaintenanceWindows:     item.MaintenanceWindows,
	}
}

func (s *TaskExportService) toUpsertInput(t ExportedTask, targetsByName, nodesByName map[string]uint, deps []uint) BackupTaskUpsertInput {
	return BackupTaskUpsertInput{
		Name:                   t.Name,
		Type:                   t.Type,
		Enabled:                t.Enabled,
		CronExpr:               t.CronExpr,
		SourcePath:             t.SourcePath,
		SourcePaths:            t.SourcePaths,
		ExcludePatterns:        t.ExcludePatterns,
		DBHost:                 t.DBHost,
		DBPort:                 t.DBPort,
		DBUser:                 t.DBUser,
		DBName:                 t.DBName,
		DBPath:                 t.DBPath,
		ExtraConfig:            t.ExtraConfig,
		StorageTargetIDs:       idsFromNames(t.StorageTargetNames, targetsByName),
		ReplicationTargetIDs:   idsFromNames(t.ReplicationTargetNames, targetsByName),
		NodeID:                 nodesByName[t.NodeName],
		Tags:                   t.Tags,
		Compression:            t.Compression,
		Encrypt:                t.Encrypt,
		RetentionDays:          t.RetentionDays,
		MaxBackups:             t.MaxBackups,
		VerifyEnabled:          t.VerifyEnabled,
		VerifyCronExpr:         t.VerifyCronExpr,
		VerifyMode:             t.VerifyMode,
		SLAHoursRPO:            t.SLAHoursRPO,
		AlertOnConsecutiveFails: t.AlertOnConsecutiveFails,
		MaintenanceWindows:     t.MaintenanceWindows,
		DependsOnTaskIDs:       deps,
	}
}

func namesFromIDs(ids []uint, lookup map[uint]string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := lookup[id]; ok {
			out = append(out, name)
		}
	}
	return out
}

func idsFromNames(names []string, lookup map[string]uint) []uint {
	out := make([]uint, 0, len(names))
	for _, name := range names {
		if id, ok := lookup[name]; ok {
			out = append(out, id)
		}
	}
	return out
}
