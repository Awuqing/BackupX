import { describe, expect, it } from 'vitest'
import {
  getBackupTaskStatusColor,
  getBackupTaskStatusLabel,
  getBackupTaskTypeLabel,
  getDefaultPort,
  isDatabaseBackupTask,
  isFileBackupTask,
  isSQLiteBackupTask,
} from './field-config'

describe('backup task field config', () => {
  it('returns readable task labels', () => {
    expect(getBackupTaskTypeLabel('file')).toBe('文件目录')
    expect(getBackupTaskTypeLabel('postgresql')).toBe('PostgreSQL')
  })

  it('classifies task types correctly', () => {
    expect(isFileBackupTask('file')).toBe(true)
    expect(isSQLiteBackupTask('sqlite')).toBe(true)
    expect(isDatabaseBackupTask('mysql')).toBe(true)
    expect(isDatabaseBackupTask('postgresql')).toBe(true)
    expect(isDatabaseBackupTask('file')).toBe(false)
  })

  it('returns expected status meta and default ports', () => {
    expect(getBackupTaskStatusLabel('success')).toBe('成功')
    expect(getBackupTaskStatusColor('failed')).toBe('red')
    expect(getDefaultPort('mysql')).toBe(3306)
    expect(getDefaultPort('postgresql')).toBe(5432)
  })
})
