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

export async function fetchSystemInfo() {
  const response = await http.get<{ code: string; message: string; data: SystemInfo }>('/system/info')
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
