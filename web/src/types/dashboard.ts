import type { BackupRecordSummary } from './backup-records'

export interface DashboardStorageUsageItem {
  storageTargetId: number
  targetName: string
  totalSize: number
}

export interface BackupTimelinePoint {
  date: string
  total: number
  success: number
  failed: number
}

export interface DashboardStats {
  totalTasks: number
  enabledTasks: number
  totalRecords: number
  successRate: number
  totalBackupBytes: number
  lastBackupAt?: string
  recentRecords: BackupRecordSummary[]
  storageUsage: DashboardStorageUsageItem[]
}
