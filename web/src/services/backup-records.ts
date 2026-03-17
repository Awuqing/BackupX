import { http, getAccessToken, type ApiEnvelope, unwrapApiEnvelope } from './http'
import type { BackupLogEvent, BackupRecordDetail, BackupRecordListFilter, BackupRecordSummary } from '../types/backup-records'
import { resolveErrorMessage } from '../utils/error'

interface RecordLogStreamHandlers {
  onEvent: (event: BackupLogEvent) => void
  onDone?: () => void
  onError?: (message: string) => void
}

function buildRecordQuery(filter: BackupRecordListFilter) {
  const query: Record<string, string | number> = {}
  if (filter.taskId) {
    query.taskId = filter.taskId
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

function parseContentDisposition(value?: string) {
  if (!value) {
    return 'backup-artifact.bin'
  }
  const match = value.match(/filename="?([^";]+)"?/i)
  return match?.[1] ?? 'backup-artifact.bin'
}

function parseLogEvent(chunk: string) {
  const payloadLine = chunk
    .split('\n')
    .find((line) => line.startsWith('data:'))

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

export async function listBackupRecords(filter: BackupRecordListFilter = {}) {
  const response = await http.get<ApiEnvelope<BackupRecordSummary[]>>('/backup/records', { params: buildRecordQuery(filter) })
  return unwrapApiEnvelope(response.data)
}

export async function getBackupRecord(id: number) {
  const response = await http.get<ApiEnvelope<BackupRecordDetail>>(`/backup/records/${id}`)
  return unwrapApiEnvelope(response.data)
}

export async function downloadBackupRecord(id: number) {
  const response = await http.get<Blob>(`/backup/records/${id}/download`, { responseType: 'blob' })
  return {
    blob: response.data,
    fileName: parseContentDisposition(response.headers['content-disposition']),
  }
}

export async function restoreBackupRecord(id: number) {
  const response = await http.post<ApiEnvelope<{ restored: boolean }>>(`/backup/records/${id}/restore`)
  return unwrapApiEnvelope(response.data)
}

export async function deleteBackupRecord(id: number) {
  const response = await http.delete<ApiEnvelope<{ deleted: boolean }>>(`/backup/records/${id}`)
  return unwrapApiEnvelope(response.data)
}

export function streamBackupRecordLogs(recordId: number, handlers: RecordLogStreamHandlers) {
  const controller = new AbortController()

  void (async () => {
    try {
      const token = getAccessToken()
      const response = await fetch(`/api/backup/records/${recordId}/logs/stream`, {
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
