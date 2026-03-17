export type BackupTaskType = 'file' | 'mysql' | 'sqlite' | 'postgresql'
export type BackupTaskStatus = 'idle' | 'running' | 'success' | 'failed'
export type BackupCompression = 'gzip' | 'none'

export interface BackupTaskSummary {
  id: number
  name: string
  type: BackupTaskType
  enabled: boolean
  cronExpr: string
  storageTargetId: number
  storageTargetName: string
  nodeId: number
  nodeName?: string
  tags: string
  retentionDays: number
  compression: BackupCompression
  encrypt: boolean
  maxBackups: number
  lastRunAt?: string
  lastStatus: BackupTaskStatus
  updatedAt: string
}

export interface BackupTaskDetail extends BackupTaskSummary {
  sourcePath: string
  excludePatterns: string[]
  dbHost: string
  dbPort: number
  dbUser: string
  dbName: string
  dbPath: string
  maskedFields?: string[]
  createdAt: string
}

export interface BackupTaskPayload {
  name: string
  type: BackupTaskType
  enabled: boolean
  cronExpr: string
  sourcePath: string
  excludePatterns: string[]
  dbHost: string
  dbPort: number
  dbUser: string
  dbPassword: string
  dbName: string
  dbPath: string
  storageTargetId: number
  nodeId: number
  tags: string
  retentionDays: number
  compression: BackupCompression
  encrypt: boolean
  maxBackups: number
}

export interface BackupTaskTogglePayload {
  enabled?: boolean
}
