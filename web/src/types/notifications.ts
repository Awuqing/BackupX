export type NotificationType = 'email' | 'webhook' | 'telegram'
export type NotificationFieldType = 'input' | 'password' | 'number' | 'textarea'

export interface NotificationSummary {
  id: number
  name: string
  type: NotificationType
  enabled: boolean
  onSuccess: boolean
  onFailure: boolean
  updatedAt: string
}

export interface NotificationDetail extends NotificationSummary {
  config: Record<string, string | number>
  maskedFields?: string[]
}

export interface NotificationPayload {
  name: string
  type: NotificationType
  enabled: boolean
  onSuccess: boolean
  onFailure: boolean
  config: Record<string, string | number>
}

export interface NotificationFieldConfig {
  key: string
  label: string
  type: NotificationFieldType
  required?: boolean
  placeholder?: string
  description?: string
  sensitive?: boolean
}
