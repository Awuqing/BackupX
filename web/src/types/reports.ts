export type ComplianceRisk = 'ok' | 'at_risk' | 'not_applicable'

export interface ComplianceTaskRow {
  taskId: number
  taskName: string
  type: string
  enabled: boolean
  nodeName: string
  cronExpr: string
  encrypted: boolean
  retentionDays: number
  slaHoursRpo: number
  totalRuns: number
  successes: number
  failures: number
  successRate: number
  lastStatus: string
  lastRunAt?: string
  lastSuccessAt?: string
  protectedBytes: number
  compliant: boolean
  risk: ComplianceRisk
}

export interface ComplianceSummary {
  totalTasks: number
  enabledTasks: number
  compliantTasks: number
  atRiskTasks: number
  encryptedTasks: number
  overallSuccessRate: number
  totalProtectedBytes: number
}

export interface ComplianceReport {
  generatedAt: string
  rangeDays: number
  summary: ComplianceSummary
  tasks: ComplianceTaskRow[]
}
