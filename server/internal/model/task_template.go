package model

import "time"

// TaskTemplate 是批量创建任务的模板。
// 用途：大规模场景（100+ 任务）下保存一份参数预设，
// 再通过"应用模板"接口一次性创建多个任务（变量替换 Name/SourcePath 等）。
//
// 参数存 JSON（Payload），结构与 service.BackupTaskUpsertInput 基本一致，
// 仅以下字段在应用时可被变量覆盖：
//   - name
//   - sourcePath / sourcePaths 中的 {{.Host}} / {{.Env}} 等占位符
type TaskTemplate struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;uniqueIndex;not null" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	TaskType    string    `gorm:"column:task_type;size:20;not null" json:"taskType"`
	// Payload JSON，存完整 BackupTaskUpsertInput 的序列化
	Payload   string    `gorm:"type:text;not null" json:"payload"`
	CreatedBy string    `gorm:"column:created_by;size:128" json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (TaskTemplate) TableName() string {
	return "task_templates"
}
