import { http, getAccessToken } from './http'
import type { AuditLogListResult } from '../types/audit'

export interface AuditListParams {
  category?: string
  action?: string
  username?: string
  targetId?: string
  keyword?: string
  dateFrom?: string
  dateTo?: string
  limit?: number
  offset?: number
}

export async function listAuditLogs(params: AuditListParams) {
  const response = await http.get<{ code: string; message: string; data: AuditLogListResult }>('/audit-logs', { params })
  return response.data.data
}

// exportAuditLogs 触发浏览器下载 CSV。
// fetch 走 token 认证，返回 blob；默认 10000 行上限。
export async function exportAuditLogs(params: AuditListParams) {
  const token = getAccessToken()
  const query = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '' && v !== null) {
      query.set(k, String(v))
    }
  }
  const response = await fetch(`/api/audit-logs/export?${query.toString()}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })
  if (!response.ok) {
    throw new Error(`导出失败 (HTTP ${response.status})`)
  }
  const blob = await response.blob()
  const cd = response.headers.get('content-disposition') ?? ''
  const match = cd.match(/filename="?([^";]+)"?/i)
  const filename = match?.[1] ?? `backupx-audit-${new Date().toISOString().slice(0, 10)}.csv`
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}
