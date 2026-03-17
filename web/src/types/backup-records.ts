export type BackupRecordStatus = 'running' | 'success' | 'failed'

export interface BackupLogEvent {
  recordId: number
  sequence: number
  level: string
  message: string
  timestamp: string
  completed: boolean
  status: string
}

export interface BackupRecordSummary {
  id: number
  taskId: number
  taskName: string
  storageTargetId: number
  storageTargetName: string
  status: BackupRecordStatus
  fileName: string
  fileSize: number
  storagePath: string
  durationSeconds: number
  errorMessage: string
  startedAt: string
  completedAt?: string
}

export interface BackupRecordDetail extends BackupRecordSummary {
  logContent: string
  logEvents?: BackupLogEvent[]
}

export interface BackupRecordListFilter {
  taskId?: number
  status?: BackupRecordStatus | ''
  dateFrom?: string
  dateTo?: string
}
