import type { BackupLogEvent } from './backup-records'

export type VerificationRecordStatus = 'running' | 'success' | 'failed'
export type VerificationMode = 'quick' | 'deep'

export interface VerificationRecordSummary {
  id: number
  backupRecordId: number
  taskId: number
  taskName: string
  nodeId: number
  mode: VerificationMode
  status: VerificationRecordStatus
  summary: string
  errorMessage: string
  durationSeconds: number
  startedAt: string
  completedAt?: string
  triggeredBy: string
  backupFileName?: string
}

export interface VerificationRecordDetail extends VerificationRecordSummary {
  logContent: string
  logEvents?: BackupLogEvent[]
}

export interface VerificationRecordListFilter {
  taskId?: number
  backupRecordId?: number
  status?: VerificationRecordStatus | ''
  dateFrom?: string
  dateTo?: string
}
