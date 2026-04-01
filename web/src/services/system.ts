import { http } from './http'

export interface SystemInfo {
  version: string
  mode: string
  startedAt: string
  uptimeSeconds: number
  databasePath: string
  diskTotal: number
  diskFree: number
  diskUsed: number
}

export interface UpdateCheckResult {
  currentVersion: string
  latestVersion: string
  hasUpdate: boolean
  releaseUrl?: string
  releaseNotes?: string
  publishedAt?: string
  downloadUrl?: string
  dockerImage?: string
  error?: string
}

export async function fetchSystemInfo() {
  const response = await http.get<{ code: string; message: string; data: SystemInfo }>('/system/info')
  return response.data.data
}

export async function checkUpdate() {
  const response = await http.get<{ code: string; message: string; data: UpdateCheckResult }>('/system/update-check')
  return response.data.data
}

export interface UpdateApplyResult {
  success: boolean
  message: string
  output?: string
}

export async function applyUpdate(version: string) {
  const response = await http.post<{ code: string; message: string; data: UpdateApplyResult }>('/system/update-apply', { version })
  return response.data.data
}

export async function fetchSettings() {
  const response = await http.get<{ code: string; message: string; data: Record<string, string> }>('/settings')
  return response.data.data
}

export async function updateSettings(settings: Record<string, string>) {
  const response = await http.put<{ code: string; message: string; data: Record<string, string> }>('/settings', settings)
  return response.data.data
}
