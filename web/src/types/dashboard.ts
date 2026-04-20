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

export interface SLAViolation {
  taskId: number
  taskName: string
  nodeId: number
  nodeName?: string
  slaHoursRpo: number
  lastSuccessAt?: string
  hoursSinceLastSuccess: number
  neverSucceeded: boolean
}

export interface SLAComplianceReport {
  totalTasksWithSla: number
  compliant: number
  violated: number
  coverageRate: number
  violations: SLAViolation[]
}

export interface ClusterNodeSummary {
  id: number
  name: string
  hostname: string
  status: 'online' | 'offline'
  isLocal: boolean
  agentVersion: string
  versionStatus: 'current' | 'outdated' | 'unknown'
  lastSeen: string
  taskCount: number
}

export interface ClusterOverview {
  masterVersion: string
  totalNodes: number
  onlineNodes: number
  offlineNodes: number
  outdatedAgents: number
  nodes: ClusterNodeSummary[]
}

export interface BreakdownItem {
  key: string
  label: string
  count?: number
  totalSize?: number
}

export interface BreakdownStats {
  byType: BreakdownItem[]
  byStatus: BreakdownItem[]
  byNode: BreakdownItem[]
  byStorage: BreakdownItem[]
}

export interface NodePerformance {
  nodeId: number
  nodeName: string
  isLocal: boolean
  totalRuns: number
  successRuns: number
  failedRuns: number
  successRate: number
  totalBytes: number
  avgDurationSecs: number
}
