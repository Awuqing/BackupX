import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { BackupTimelinePoint, DashboardStats } from '../types/dashboard'

export async function fetchDashboardStats() {
  const response = await http.get<ApiEnvelope<DashboardStats>>('/dashboard/stats')
  return unwrapApiEnvelope(response.data)
}

export async function fetchDashboardTimeline(days = 30) {
  const response = await http.get<ApiEnvelope<BackupTimelinePoint[]>>('/dashboard/timeline', { params: { days } })
  return unwrapApiEnvelope(response.data)
}
