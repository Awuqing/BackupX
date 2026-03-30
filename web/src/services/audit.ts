import { http } from './http'
import type { AuditLogListResult } from '../types/audit'

export async function listAuditLogs(params: { category?: string; limit?: number; offset?: number }) {
  const response = await http.get<{ code: string; message: string; data: AuditLogListResult }>('/audit-logs', { params })
  return response.data.data
}
