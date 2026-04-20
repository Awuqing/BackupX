import { http, getAccessToken, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { BackupLogEvent } from '../types/backup-records'
import type { RestoreRecordDetail, RestoreRecordListFilter, RestoreRecordSummary } from '../types/restore-records'
import { resolveErrorMessage } from '../utils/error'

interface RestoreLogStreamHandlers {
  onEvent: (event: BackupLogEvent) => void
  onDone?: () => void
  onError?: (message: string) => void
}

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
  const response = await http.get<ApiEnvelope<RestoreRecordSummary[]>>('/restore/records', { params: buildQuery(filter) })
  return unwrapApiEnvelope(response.data)
}

export async function getRestoreRecord(id: number) {
  const response = await http.get<ApiEnvelope<RestoreRecordDetail>>(`/restore/records/${id}`)
  return unwrapApiEnvelope(response.data)
}

// startRestoreFromBackup 通过源备份记录启动恢复。返回新建的恢复记录详情。
export async function startRestoreFromBackup(backupRecordId: number) {
  const response = await http.post<ApiEnvelope<RestoreRecordDetail>>(`/backup/records/${backupRecordId}/restore`)
  return unwrapApiEnvelope(response.data)
}

function parseLogEvent(chunk: string) {
  const payloadLine = chunk.split('\n').find((line) => line.startsWith('data:'))
  if (!payloadLine) {
    return null
  }
  const payload = payloadLine.slice(5).trim()
  if (!payload) {
    return null
  }
  return JSON.parse(payload) as BackupLogEvent
}

async function resolveStreamError(response: Response) {
  try {
    const payload = (await response.json()) as { message?: string }
    return payload.message ?? '连接日志流失败'
  } catch {
    return `连接日志流失败（HTTP ${response.status}）`
  }
}

export function streamRestoreRecordLogs(restoreId: number, handlers: RestoreLogStreamHandlers) {
  const controller = new AbortController()

  void (async () => {
    try {
      const token = getAccessToken()
      const response = await fetch(`/api/restore/records/${restoreId}/logs/stream`, {
        method: 'GET',
        headers: token ? { Authorization: `Bearer ${token}` } : undefined,
        signal: controller.signal,
      })

      if (!response.ok) {
        throw new Error(await resolveStreamError(response))
      }
      if (!response.body) {
        throw new Error('日志流不可用')
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { value, done } = await reader.read()
        if (done) {
          break
        }

        buffer += decoder.decode(value, { stream: true })

        while (buffer.includes('\n\n')) {
          const boundary = buffer.indexOf('\n\n')
          const chunk = buffer.slice(0, boundary)
          buffer = buffer.slice(boundary + 2)

          const event = parseLogEvent(chunk)
          if (!event) {
            continue
          }
          handlers.onEvent(event)
          if (event.completed) {
            handlers.onDone?.()
            controller.abort()
            return
          }
        }
      }

      if (buffer.trim()) {
        const event = parseLogEvent(buffer)
        if (event) {
          handlers.onEvent(event)
        }
      }
      handlers.onDone?.()
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        return
      }
      handlers.onError?.(resolveErrorMessage(error, '日志流连接失败'))
    }
  })()

  return () => controller.abort()
}
