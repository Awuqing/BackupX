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
  checksum: string
  storagePath: string
  durationSeconds: number
  errorMessage: string
  startedAt: string
  completedAt?: string
}

export interface StorageUploadResultItem {
  storageTargetId: number
  storageTargetName: string
  status: 'success' | 'failed'
  storagePath?: string
  fileSize?: number
  error?: string
}

export interface BackupRecordDetail extends BackupRecordSummary {
  logContent: string
  logEvents?: BackupLogEvent[]
  storageUploadResults?: StorageUploadResultItem[]
}

export interface BackupRecordListFilter {
  taskId?: number
  status?: BackupRecordStatus | ''
  dateFrom?: string
  dateTo?: string
}
