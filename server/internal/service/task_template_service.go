package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// TaskTemplateService 管理任务模板 + 一键批量创建任务。
type TaskTemplateService struct {
	templates repository.TaskTemplateRepository
	tasks     *BackupTaskService
}

func NewTaskTemplateService(templates repository.TaskTemplateRepository, tasks *BackupTaskService) *TaskTemplateService {
	return &TaskTemplateService{templates: templates, tasks: tasks}
}

type TaskTemplateSummary struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TaskType    string `json:"taskType"`
	CreatedBy   string `json:"createdBy"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type TaskTemplateDetail struct {
	TaskTemplateSummary
	Payload BackupTaskUpsertInput `json:"payload"`
}

// TaskTemplateUpsertInput 创建/更新模板时的输入。
// Payload 字段与 BackupTaskUpsertInput 复用同一结构。
type TaskTemplateUpsertInput struct {
	Name        string                `json:"name" binding:"required,min=1,max=128"`
	Description string                `json:"description" binding:"max=500"`
	Payload     BackupTaskUpsertInput `json:"payload" binding:"required"`
}

// TaskTemplateApplyInput 应用模板批量创建任务。
// 每个 Variables 条目会用 Variables 中的字段覆盖模板 Payload 生成一个新任务：
//   - name 必填（覆盖模板 Name，任务命名）
//   - sourcePath / sourcePaths / dbHost / dbName 若提供则覆盖
type TaskTemplateApplyInput struct {
	Variables []TaskTemplateVariables `json:"variables" binding:"required,min=1,max=100"`
}

type TaskTemplateVariables struct {
	Name        string   `json:"name" binding:"required,min=1,max=100"`
	SourcePath  string   `json:"sourcePath"`
	SourcePaths []string `json:"sourcePaths"`
	DBHost      string   `json:"dbHost"`
	DBName      string   `json:"dbName"`
	Tags        string   `json:"tags"`
	NodeID      *uint    `json:"nodeId"`
}

// TaskTemplateApplyResult 单个任务的创建结果。
type TaskTemplateApplyResult struct {
	Name    string `json:"name"`
	TaskID  uint   `json:"taskId,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func (s *TaskTemplateService) List(ctx context.Context) ([]TaskTemplateSummary, error) {
	items, err := s.templates.List(ctx)
	if err != nil {
		return nil, apperror.Internal("TASK_TEMPLATE_LIST_FAILED", "无法获取任务模板列表", err)
	}
	result := make([]TaskTemplateSummary, 0, len(items))
	for i := range items {
		result = append(result, toTemplateSummary(&items[i]))
	}
	return result, nil
}

func (s *TaskTemplateService) Get(ctx context.Context, id uint) (*TaskTemplateDetail, error) {
	item, err := s.templates.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("TASK_TEMPLATE_GET_FAILED", "无法获取任务模板", err)
	}
	if item == nil {
		return nil, apperror.New(404, "TASK_TEMPLATE_NOT_FOUND", "任务模板不存在", nil)
	}
	var payload BackupTaskUpsertInput
	if err := json.Unmarshal([]byte(item.Payload), &payload); err != nil {
		return nil, apperror.Internal("TASK_TEMPLATE_DECODE_FAILED", "无法解析模板内容", err)
	}
	detail := &TaskTemplateDetail{TaskTemplateSummary: toTemplateSummary(item), Payload: payload}
	return detail, nil
}

func (s *TaskTemplateService) Create(ctx context.Context, createdBy string, input TaskTemplateUpsertInput) (*TaskTemplateDetail, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, apperror.BadRequest("TASK_TEMPLATE_INVALID", "名称不能为空", nil)
	}
	existing, err := s.templates.FindByName(ctx, strings.TrimSpace(input.Name))
	if err != nil {
		return nil, apperror.Internal("TASK_TEMPLATE_LOOKUP_FAILED", "无法校验模板名", err)
	}
	if existing != nil {
		return nil, apperror.Conflict("TASK_TEMPLATE_NAME_EXISTS", "模板名称已存在", nil)
	}
	payloadJSON, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, apperror.Internal("TASK_TEMPLATE_ENCODE_FAILED", "无法序列化模板参数", err)
	}
	item := &model.TaskTemplate{
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		TaskType:    strings.TrimSpace(input.Payload.Type),
		Payload:     string(payloadJSON),
		CreatedBy:   strings.TrimSpace(createdBy),
	}
	if err := s.templates.Create(ctx, item); err != nil {
		return nil, apperror.Internal("TASK_TEMPLATE_CREATE_FAILED", "无法创建任务模板", err)
	}
	return s.Get(ctx, item.ID)
}

func (s *TaskTemplateService) Update(ctx context.Context, id uint, input TaskTemplateUpsertInput) (*TaskTemplateDetail, error) {
	item, err := s.templates.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("TASK_TEMPLATE_GET_FAILED", "无法获取任务模板", err)
	}
	if item == nil {
		return nil, apperror.New(404, "TASK_TEMPLATE_NOT_FOUND", "任务模板不存在", nil)
	}
	payloadJSON, err := json.Marshal(input.Payload)
	if err != nil {
		return nil, apperror.Internal("TASK_TEMPLATE_ENCODE_FAILED", "无法序列化模板参数", err)
	}
	if strings.TrimSpace(input.Name) != item.Name {
		dup, err := s.templates.FindByName(ctx, strings.TrimSpace(input.Name))
		if err != nil {
			return nil, apperror.Internal("TASK_TEMPLATE_LOOKUP_FAILED", "无法校验模板名", err)
		}
		if dup != nil && dup.ID != id {
			return nil, apperror.Conflict("TASK_TEMPLATE_NAME_EXISTS", "模板名称已存在", nil)
		}
	}
	item.Name = strings.TrimSpace(input.Name)
	item.Description = strings.TrimSpace(input.Description)
	item.TaskType = strings.TrimSpace(input.Payload.Type)
	item.Payload = string(payloadJSON)
	if err := s.templates.Update(ctx, item); err != nil {
		return nil, apperror.Internal("TASK_TEMPLATE_UPDATE_FAILED", "无法更新任务模板", err)
	}
	return s.Get(ctx, item.ID)
}

func (s *TaskTemplateService) Delete(ctx context.Context, id uint) error {
	item, err := s.templates.FindByID(ctx, id)
	if err != nil {
		return apperror.Internal("TASK_TEMPLATE_GET_FAILED", "无法获取任务模板", err)
	}
	if item == nil {
		return apperror.New(404, "TASK_TEMPLATE_NOT_FOUND", "任务模板不存在", nil)
	}
	return s.templates.Delete(ctx, id)
}

// Apply 从模板批量创建任务。best-effort：单个失败不影响其他。
// 每个 Variables 条目按 name 覆盖任务名；其他字段（sourcePath/dbHost/dbName/tags/nodeId）非空则覆盖模板对应字段。
func (s *TaskTemplateService) Apply(ctx context.Context, id uint, input TaskTemplateApplyInput) ([]TaskTemplateApplyResult, error) {
	template, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if s.tasks == nil {
		return nil, apperror.Internal("TASK_TEMPLATE_APPLY_UNAVAILABLE", "任务创建服务未注入", nil)
	}
	results := make([]TaskTemplateApplyResult, 0, len(input.Variables))
	for _, v := range input.Variables {
		payload := mergeVariables(template.Payload, v)
		detail, createErr := s.tasks.Create(ctx, payload)
		result := TaskTemplateApplyResult{Name: v.Name}
		if createErr != nil {
			result.Success = false
			if appErr, ok := createErr.(*apperror.AppError); ok {
				result.Error = appErr.Message
			} else {
				result.Error = createErr.Error()
			}
		} else {
			result.Success = true
			result.TaskID = detail.ID
		}
		results = append(results, result)
	}
	return results, nil
}

// mergeVariables 把 Variables 覆盖到模板 Payload 上。返回一个新的 Input（不污染模板）。
func mergeVariables(base BackupTaskUpsertInput, v TaskTemplateVariables) BackupTaskUpsertInput {
	out := base
	out.Name = strings.TrimSpace(v.Name)
	if strings.TrimSpace(v.SourcePath) != "" {
		out.SourcePath = strings.TrimSpace(v.SourcePath)
	}
	if len(v.SourcePaths) > 0 {
		out.SourcePaths = v.SourcePaths
	}
	if strings.TrimSpace(v.DBHost) != "" {
		out.DBHost = strings.TrimSpace(v.DBHost)
	}
	if strings.TrimSpace(v.DBName) != "" {
		out.DBName = strings.TrimSpace(v.DBName)
	}
	if strings.TrimSpace(v.Tags) != "" {
		out.Tags = strings.TrimSpace(v.Tags)
	}
	if v.NodeID != nil {
		out.NodeID = *v.NodeID
	}
	return out
}

func toTemplateSummary(item *model.TaskTemplate) TaskTemplateSummary {
	return TaskTemplateSummary{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		TaskType:    item.TaskType,
		CreatedBy:   item.CreatedBy,
		CreatedAt:   item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   item.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// 确保未使用告警
var _ = fmt.Sprintf
