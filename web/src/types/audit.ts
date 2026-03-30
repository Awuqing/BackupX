export interface AuditLog {
  id: number
  userId: number
  username: string
  category: string
  action: string
  targetType: string
  targetId: string
  targetName: string
  detail: string
  clientIp: string
  createdAt: string
}

export interface AuditLogListResult {
  items: AuditLog[]
  total: number
}
