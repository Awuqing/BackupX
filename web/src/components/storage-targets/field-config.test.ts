import { describe, expect, it } from 'vitest'
import { getStorageTargetFieldConfigs, getStorageTargetTypeLabel } from './field-config'

describe('storage target field config', () => {
  it('returns local disk field config', () => {
    const fields = getStorageTargetFieldConfigs('local_disk')
    expect(fields).toHaveLength(1)
    expect(fields[0]?.key).toBe('basePath')
  })

  it('returns readable type labels', () => {
    expect(getStorageTargetTypeLabel('google_drive')).toBe('Google Drive')
    expect(getStorageTargetTypeLabel('webdav')).toBe('WebDAV')
  })
})
