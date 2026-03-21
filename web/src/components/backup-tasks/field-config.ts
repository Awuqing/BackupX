import type { BackupCompression, BackupTaskStatus, BackupTaskType } from '../../types/backup-tasks'

export const backupTaskTypeOptions = [
  { label: '文件目录', value: 'file' },
  { label: 'MySQL', value: 'mysql' },
  { label: 'SQLite', value: 'sqlite' },
  { label: 'PostgreSQL', value: 'postgresql' },
  { label: 'SAP HANA', value: 'saphana' },
] as const

export const backupCompressionOptions = [
  { label: 'Gzip 压缩', value: 'gzip' },
  { label: '不压缩', value: 'none' },
] as const

export function getBackupTaskTypeLabel(type: BackupTaskType) {
  switch (type) {
    case 'file':
      return '文件目录'
    case 'mysql':
      return 'MySQL'
    case 'sqlite':
      return 'SQLite'
    case 'postgresql':
      return 'PostgreSQL'
    case 'saphana':
      return 'SAP HANA'
    default:
      return type
  }
}

export function getBackupTaskStatusLabel(status: BackupTaskStatus) {
  switch (status) {
    case 'idle':
      return '空闲'
    case 'running':
      return '执行中'
    case 'success':
      return '成功'
    case 'failed':
      return '失败'
    default:
      return status
  }
}

export function getBackupTaskStatusColor(status: BackupTaskStatus) {
  switch (status) {
    case 'success':
      return 'green'
    case 'failed':
      return 'red'
    case 'running':
      return 'arcoblue'
    default:
      return 'gray'
  }
}

export function isFileBackupTask(type: BackupTaskType) {
  return type === 'file'
}

export function isSQLiteBackupTask(type: BackupTaskType) {
  return type === 'sqlite'
}

export function isDatabaseBackupTask(type: BackupTaskType) {
  return type === 'mysql' || type === 'postgresql' || type === 'saphana'
}

export function getDefaultPort(type: BackupTaskType) {
  switch (type) {
    case 'mysql':
      return 3306
    case 'postgresql':
      return 5432
    case 'saphana':
      return 30015
    default:
      return 0
  }
}

export function getCompressionLabel(compression: BackupCompression) {
  return compression === 'gzip' ? 'Gzip' : '无'
}
