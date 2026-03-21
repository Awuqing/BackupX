export type StorageTargetType = 'local_disk' | 'google_drive' | 's3' | 'webdav' | 'aliyun_oss' | 'tencent_cos' | 'qiniu_kodo' | 'ftp'
export type StorageTestStatus = 'unknown' | 'success' | 'failed'
export type StorageFieldType = 'input' | 'password' | 'switch'

export interface StorageTargetSummary {
  id: number
  name: string
  type: StorageTargetType
  description: string
  enabled: boolean
  updatedAt: string
  lastTestedAt?: string
  lastTestStatus: StorageTestStatus
  lastTestMessage?: string
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
