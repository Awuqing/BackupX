import { http, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { BackupLogEvent } from '../types/backup-records'
import type {
  RestoreRecordDetail,
  RestoreRecordListFilter,
  RestoreRecordSummary,
} from '../types/restore-records'
import { streamLogEvents, type LogStreamHandlers } from './log-stream'

function buildQuery(filter: RestoreRecordListFilter) {
  const query: Record<string, string | number> = {}
  if (filter.taskId) {
    query.taskId = filter.taskId
  }
  if (filter.backupRecordId) {
    query.backupRecordId = filter.backupRecordId
  }
  if (filter.status) {
    query.status = filter.status
  }
  if (filter.dateFrom) {
    query.dateFrom = filter.dateFrom
  }
  if (filter.dateTo) {
    query.dateTo = filter.dateTo
  }
  return query
}

export async function listRestoreRecords(filter: RestoreRecordListFilter = {}) {
  const response = await http.get<ApiEnvelope<RestoreRecordSummary[]>>('/restore/records', {
    params: buildQuery(filter),
  })
  return unwrapApiEnvelope(response.data)
}

export async function getRestoreRecord(id: number) {
  const response = await http.get<ApiEnvelope<RestoreRecordDetail>>(`/restore/records/${id}`)
  return unwrapApiEnvelope(response.data)
}

// startRestoreFromBackup 通过源备份记录启动恢复。两个可选项互不影响：
//   - selectedPaths 非空时为按需（选择性）恢复，仅还原选中的文件/目录；
//   - targetPath 非空时把文件归档恢复到该绝对目录而非原始路径（仅文件类型本机恢复）。
// 返回新建的恢复记录详情。
export async function startRestoreFromBackup(
  backupRecordId: number,
  selectedPaths?: string[],
  targetPath?: string,
  restoreTime?: string,
) {
  const body: { selectedPaths?: string[]; targetPath?: string; restoreTime?: string } = {}
  if (selectedPaths && selectedPaths.length > 0) {
    body.selectedPaths = selectedPaths
  }
  if (targetPath && targetPath.trim()) {
    body.targetPath = targetPath.trim()
  }
  if (restoreTime) body.restoreTime = restoreTime
  const response = await http.post<ApiEnvelope<RestoreRecordDetail>>(
    `/backup/records/${backupRecordId}/restore`,
    body,
  )
  return unwrapApiEnvelope(response.data)
}

export function streamRestoreRecordLogs(
  restoreId: number,
  handlers: LogStreamHandlers<BackupLogEvent>,
) {
  return streamLogEvents(`/api/restore/records/${restoreId}/logs/stream`, handlers)
}
