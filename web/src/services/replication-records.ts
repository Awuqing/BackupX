import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'

export type ReplicationStatus = 'running' | 'success' | 'failed'

export interface ReplicationRecordSummary {
  id: number
  backupRecordId: number
  taskId: number
  sourceTargetId: number
  sourceTargetName: string
  destTargetId: number
  destTargetName: string
  status: ReplicationStatus
  storagePath: string
  fileSize: number
  checksum: string
  errorMessage: string
  durationSeconds: number
  triggeredBy: string
  startedAt: string
  completedAt?: string
}

export interface ReplicationListFilter {
  taskId?: number
  backupRecordId?: number
  destTargetId?: number
  status?: ReplicationStatus | ''
}

function buildQuery(filter: ReplicationListFilter) {
  const q: Record<string, string | number> = {}
  if (filter.taskId) q.taskId = filter.taskId
  if (filter.backupRecordId) q.backupRecordId = filter.backupRecordId
  if (filter.destTargetId) q.destTargetId = filter.destTargetId
  if (filter.status) q.status = filter.status
  return q
}

export async function listReplicationRecords(filter: ReplicationListFilter = {}) {
  const response = await http.get<ApiEnvelope<ReplicationRecordSummary[]>>('/replication/records', { params: buildQuery(filter) })
  return unwrapApiEnvelope(response.data)
}

export async function getReplicationRecord(id: number) {
  const response = await http.get<ApiEnvelope<ReplicationRecordSummary>>(`/replication/records/${id}`)
  return unwrapApiEnvelope(response.data)
}

export async function startReplication(backupRecordId: number, destTargetId: number) {
  const response = await http.post<ApiEnvelope<ReplicationRecordSummary>>(`/backup/records/${backupRecordId}/replicate`, { destTargetId })
  return unwrapApiEnvelope(response.data)
}
