import { http, getAccessToken, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { BackupTaskDetail, BackupTaskPayload, BackupTaskSummary, BackupTaskTogglePayload } from '../types/backup-tasks'
import type { BackupRecordDetail } from '../types/backup-records'

export async function listBackupTasks() {
  const response = await http.get<ApiEnvelope<BackupTaskSummary[]>>('/backup/tasks')
  return unwrapApiEnvelope(response.data)
}

export async function getBackupTask(id: number) {
  const response = await http.get<ApiEnvelope<BackupTaskDetail>>(`/backup/tasks/${id}`)
  return unwrapApiEnvelope(response.data)
}

export async function createBackupTask(payload: BackupTaskPayload) {
  const response = await http.post<ApiEnvelope<BackupTaskDetail>>('/backup/tasks', payload)
  return unwrapApiEnvelope(response.data)
}

export async function updateBackupTask(id: number, payload: BackupTaskPayload) {
  const response = await http.put<ApiEnvelope<BackupTaskDetail>>(`/backup/tasks/${id}`, payload)
  return unwrapApiEnvelope(response.data)
}

export async function deleteBackupTask(id: number) {
  const response = await http.delete<ApiEnvelope<{ deleted: boolean }>>(`/backup/tasks/${id}`)
  return unwrapApiEnvelope(response.data)
}

export async function toggleBackupTask(id: number, payload: BackupTaskTogglePayload) {
  const response = await http.put<ApiEnvelope<BackupTaskSummary>>(`/backup/tasks/${id}/toggle`, payload)
  return unwrapApiEnvelope(response.data)
}

export async function runBackupTask(id: number) {
  const response = await http.post<ApiEnvelope<BackupRecordDetail>>(`/backup/tasks/${id}/run`)
  return unwrapApiEnvelope(response.data)
}

export async function listBackupTaskTags() {
  const response = await http.get<ApiEnvelope<string[] | null>>('/backup/tasks/tags')
  return unwrapApiEnvelope(response.data) ?? []
}

// 批量操作结果
export interface BatchResult {
  id: number
  name?: string
  success: boolean
  error?: string
}

export async function batchToggleTasks(ids: number[], enabled: boolean) {
  const response = await http.post<ApiEnvelope<BatchResult[]>>('/backup/tasks/batch/toggle', { ids, enabled })
  return unwrapApiEnvelope(response.data) ?? []
}

export async function batchDeleteTasks(ids: number[]) {
  const response = await http.post<ApiEnvelope<BatchResult[]>>('/backup/tasks/batch/delete', { ids })
  return unwrapApiEnvelope(response.data) ?? []
}

export async function batchRunTasks(ids: number[]) {
  const response = await http.post<ApiEnvelope<BatchResult[]>>('/backup/tasks/batch/run', { ids })
  return unwrapApiEnvelope(response.data) ?? []
}

// 导入/导出 JSON
export interface TaskImportResult {
  name: string
  taskId?: number
  success: boolean
  error?: string
  skipped?: boolean
}

/** 导出任务配置为 JSON 文件。ids 为空则导出全部。 */
export async function exportBackupTasks(ids?: number[]): Promise<void> {
  const token = getAccessToken()
  const qs = ids && ids.length > 0 ? `?ids=${ids.join(',')}` : ''
  const response = await fetch(`/api/backup/tasks/export${qs}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })
  if (!response.ok) throw new Error(`导出失败 (HTTP ${response.status})`)
  const blob = await response.blob()
  const cd = response.headers.get('content-disposition') ?? ''
  const match = cd.match(/filename="?([^";]+)"?/i)
  const filename = match?.[1] ?? `backupx-tasks-${new Date().toISOString().slice(0, 10)}.json`
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}

export async function importBackupTasks(payload: unknown) {
  const response = await http.post<ApiEnvelope<TaskImportResult[]>>('/backup/tasks/import', payload)
  return unwrapApiEnvelope(response.data) ?? []
}
