import type { BackupLogEvent } from './backup-records'

export type RestoreRecordStatus = 'running' | 'success' | 'failed'

export interface RestoreRecordSummary {
  id: number
  backupRecordId: number
  taskId: number
  taskName: string
  nodeId: number
  nodeName?: string
  status: RestoreRecordStatus
  errorMessage: string
  durationSeconds: number
  startedAt: string
  completedAt?: string
  triggeredBy: string
  backupFileName?: string
}

export interface RestoreRecordDetail extends RestoreRecordSummary {
  logContent: string
  logEvents?: BackupLogEvent[]
}

export interface RestoreRecordListFilter {
  taskId?: number
  backupRecordId?: number
  status?: RestoreRecordStatus | ''
  dateFrom?: string
  dateTo?: string
}
