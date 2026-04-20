import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { BackupTimelinePoint, BreakdownStats, ClusterOverview, DashboardStats, NodePerformance, SLAComplianceReport } from '../types/dashboard'

export async function fetchDashboardStats() {
  const response = await http.get<ApiEnvelope<DashboardStats>>('/dashboard/stats')
  return unwrapApiEnvelope(response.data)
}

export async function fetchDashboardTimeline(days = 30) {
  const response = await http.get<ApiEnvelope<BackupTimelinePoint[]>>('/dashboard/timeline', { params: { days } })
  return unwrapApiEnvelope(response.data)
}

export async function fetchDashboardSLA() {
  const response = await http.get<ApiEnvelope<SLAComplianceReport>>('/dashboard/sla')
  return unwrapApiEnvelope(response.data)
}

export async function fetchDashboardCluster() {
  const response = await http.get<ApiEnvelope<ClusterOverview>>('/dashboard/cluster')
  return unwrapApiEnvelope(response.data)
}

export async function fetchDashboardBreakdown(days = 30) {
  const response = await http.get<ApiEnvelope<BreakdownStats>>('/dashboard/breakdown', { params: { days } })
  return unwrapApiEnvelope(response.data)
}

export async function fetchDashboardNodePerformance(days = 30) {
  const response = await http.get<ApiEnvelope<NodePerformance[]>>('/dashboard/node-performance', { params: { days } })
  return unwrapApiEnvelope(response.data) ?? []
}
