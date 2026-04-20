// 内置类型 + 全部 rclone 后端名（sftp, azureblob, dropbox 等）
export type StorageTargetType = string
export type StorageTestStatus = 'unknown' | 'success' | 'failed'
export type StorageFieldType = 'input' | 'password' | 'switch'

export interface StorageTargetSummary {
  id: number
  name: string
  type: StorageTargetType
  description: string
  enabled: boolean
  starred: boolean
  updatedAt: string
  lastTestedAt?: string
  lastTestStatus: StorageTestStatus
  lastTestMessage?: string
  /** 软配额（字节），0 = 不限制 */
  quotaBytes?: number
}

export interface StorageTargetDetail extends StorageTargetSummary {
  configVersion?: number
  config: Record<string, string | boolean>
  maskedFields?: string[]
}

export interface StorageTargetPayload {
  name: string
  type: StorageTargetType
  description: string
  enabled: boolean
  config: Record<string, string | boolean>
  /** 软配额（字节），0 = 不限制 */
  quotaBytes?: number
}

export interface StorageConnectionTestResult {
  success: boolean
  message: string
}

export interface GoogleDriveAuthStartResult {
  authUrl: string
}

export interface GoogleDriveCallbackResult {
  success: boolean
  message: string
  target?: StorageTargetDetail
}

export interface StorageTargetFieldConfig {
  key: string
  label: string
  type: StorageFieldType
  required?: boolean
  placeholder?: string
  description?: string
  sensitive?: boolean
}
