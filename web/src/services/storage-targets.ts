import { http } from './http'
import type {
  GoogleDriveAuthStartResult,
  GoogleDriveCallbackResult,
  StorageConnectionTestResult,
  StorageTargetDetail,
  StorageTargetPayload,
  StorageTargetSummary,
} from '../types/storage-targets'

interface ApiEnvelope<T> {
  code: string | number
  message: string
  data: T
}

function unwrap<T>(response: ApiEnvelope<T>) {
  return response.data
}

export async function listStorageTargets() {
  const response = await http.get<ApiEnvelope<StorageTargetSummary[]>>('/storage-targets')
  return unwrap(response.data)
}

export async function getStorageTarget(id: number) {
  const response = await http.get<ApiEnvelope<StorageTargetDetail>>(`/storage-targets/${id}`)
  return unwrap(response.data)
}

export async function createStorageTarget(payload: StorageTargetPayload) {
  const response = await http.post<ApiEnvelope<StorageTargetDetail>>('/storage-targets', payload)
  return unwrap(response.data)
}

export async function updateStorageTarget(id: number, payload: StorageTargetPayload) {
  const response = await http.put<ApiEnvelope<StorageTargetDetail>>(`/storage-targets/${id}`, payload)
  return unwrap(response.data)
}

export async function deleteStorageTarget(id: number) {
  const response = await http.delete<ApiEnvelope<{ deleted: boolean }>>(`/storage-targets/${id}`)
  return unwrap(response.data)
}

export async function testStorageTarget(payload: StorageTargetPayload) {
  const response = await http.post<ApiEnvelope<StorageConnectionTestResult>>('/storage-targets/test', payload, { timeout: 30000 })
  return unwrap(response.data)
}

export async function testSavedStorageTarget(id: number) {
  const response = await http.post<ApiEnvelope<StorageConnectionTestResult>>(`/storage-targets/${id}/test`, undefined, { timeout: 30000 })
  return unwrap(response.data)
}

export async function startGoogleDriveAuth(payload: StorageTargetPayload, targetId?: number) {
  const response = await http.post<ApiEnvelope<GoogleDriveAuthStartResult>>('/storage-targets/google-drive/auth-url', {
    ...payload,
    targetId,
  })
  return unwrap(response.data)
}

export async function completeGoogleDriveAuth(queryString: string) {
  const suffix = queryString.startsWith('?') ? queryString : `?${queryString}`
  const response = await http.get<ApiEnvelope<GoogleDriveCallbackResult>>(`/storage-targets/google-drive/callback${suffix}`)
  return unwrap(response.data)
}

export interface StorageDiskUsage {
  total?: number
  used?: number
  free?: number
  objects?: number
}

export interface StorageTargetUsage {
  targetId: number
  targetName: string
  recordCount: number
  totalSize: number
  diskUsage?: StorageDiskUsage
}

export async function toggleStorageTargetStar(id: number) {
  const response = await http.put<ApiEnvelope<StorageTargetSummary>>(`/storage-targets/${id}/star`)
  return unwrap(response.data)
}

export async function getStorageTargetUsage(id: number) {
  const response = await http.get<ApiEnvelope<StorageTargetUsage>>(`/storage-targets/${id}/usage`)
  return unwrap(response.data)
}
