import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'
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
