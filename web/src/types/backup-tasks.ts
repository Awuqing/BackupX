export type BackupTaskType = 'file' | 'mysql' | 'sqlite' | 'postgresql' | 'saphana'
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
  storageTargetIds: number[]
  storageTargetNames: string[]
  nodeId: number
  nodeName?: string
  tags: string
  retentionDays: number
  compression: BackupCompression
  encrypt: boolean
  maxBackups: number
  lastRunAt?: string
  lastStatus: BackupTaskStatus
  verifyEnabled: boolean
  verifyCronExpr: string
  verifyMode: 'quick' | 'deep'
  slaHoursRpo: number
  alertOnConsecutiveFails: number
  replicationTargetIds: number[]
  maintenanceWindows: string
  dependsOnTaskIds: number[]
  updatedAt: string
}

export interface BackupTaskDetail extends BackupTaskSummary {
  sourcePath: string
  sourcePaths: string[]
  excludePatterns: string[]
  dbHost: string
  dbPort: number
  dbUser: string
  dbName: string
  dbPath: string
  /** 类型特有的扩展配置（如 SAP HANA 的 backupLevel/backupChannels 等） */
  extraConfig?: Record<string, unknown>
  maskedFields?: string[]
  createdAt: string
}

export interface BackupTaskPayload {
  name: string
  type: BackupTaskType
  enabled: boolean
  cronExpr: string
  sourcePath: string
  sourcePaths: string[]
  excludePatterns: string[]
  dbHost: string
  dbPort: number
  dbUser: string
  dbPassword: string
  dbName: string
  dbPath: string
  storageTargetId: number
  storageTargetIds: number[]
  nodeId: number
  tags: string
  retentionDays: number
  compression: BackupCompression
  encrypt: boolean
  maxBackups: number
  /** 类型特有的扩展配置（如 SAP HANA 的 backupLevel/backupChannels 等） */
  extraConfig?: Record<string, unknown>
  verifyEnabled: boolean
  verifyCronExpr: string
  verifyMode: 'quick' | 'deep'
  slaHoursRpo: number
  alertOnConsecutiveFails: number
  replicationTargetIds: number[]
  maintenanceWindows: string
  dependsOnTaskIds: number[]
}

export interface BackupTaskTogglePayload {
  enabled?: boolean
}
