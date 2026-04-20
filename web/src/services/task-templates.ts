import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { BackupTaskPayload } from '../types/backup-tasks'

export interface TaskTemplateSummary {
  id: number
  name: string
  description: string
  taskType: string
  createdBy: string
  createdAt: string
  updatedAt: string
}

export interface TaskTemplateDetail extends TaskTemplateSummary {
  payload: BackupTaskPayload
}

export interface TaskTemplateUpsertPayload {
  name: string
  description: string
  payload: BackupTaskPayload
}

export interface TaskTemplateVariables {
  name: string
  sourcePath?: string
  sourcePaths?: string[]
  dbHost?: string
  dbName?: string
  tags?: string
  nodeId?: number
}

export interface TaskTemplateApplyResult {
  name: string
  taskId?: number
  success: boolean
  error?: string
}

export async function listTaskTemplates() {
  const response = await http.get<ApiEnvelope<TaskTemplateSummary[] | null>>('/task-templates')
  return unwrapApiEnvelope(response.data) ?? []
}

export async function getTaskTemplate(id: number) {
  const response = await http.get<ApiEnvelope<TaskTemplateDetail>>(`/task-templates/${id}`)
  return unwrapApiEnvelope(response.data)
}

export async function createTaskTemplate(payload: TaskTemplateUpsertPayload) {
  const response = await http.post<ApiEnvelope<TaskTemplateDetail>>('/task-templates', payload)
  return unwrapApiEnvelope(response.data)
}

export async function updateTaskTemplate(id: number, payload: TaskTemplateUpsertPayload) {
  const response = await http.put<ApiEnvelope<TaskTemplateDetail>>(`/task-templates/${id}`, payload)
  return unwrapApiEnvelope(response.data)
}

export async function deleteTaskTemplate(id: number) {
  const response = await http.delete<ApiEnvelope<{ deleted: boolean }>>(`/task-templates/${id}`)
  return unwrapApiEnvelope(response.data)
}

export async function applyTaskTemplate(id: number, variables: TaskTemplateVariables[]) {
  const response = await http.post<ApiEnvelope<TaskTemplateApplyResult[]>>(`/task-templates/${id}/apply`, { variables })
  return unwrapApiEnvelope(response.data)
}
