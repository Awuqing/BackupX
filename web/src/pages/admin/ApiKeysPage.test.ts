import { describe, expect, it } from 'vitest'
import type { ApiKeySummary } from '../../services/api-keys'
import { resolveApiKeyStatus } from './ApiKeysPage'

const baseKey: ApiKeySummary = {
  id: 1,
  name: 'automation',
  role: 'viewer',
  prefix: 'bax_example',
  createdBy: 'admin',
  disabled: false,
  createdAt: '2026-08-01T00:00:00Z',
}

describe('resolveApiKeyStatus', () => {
  const now = new Date('2026-08-07T00:00:00Z').getTime()

  it('derives active, disabled, and expired states from the credential lifecycle', () => {
    expect(resolveApiKeyStatus(baseKey, now)).toBe('active')
    expect(resolveApiKeyStatus({ ...baseKey, disabled: true }, now)).toBe('disabled')
    expect(resolveApiKeyStatus({ ...baseKey, expiresAt: '2026-08-06T23:59:59Z' }, now)).toBe('expired')
  })

  it('keeps expiration authoritative when an expired key is also disabled', () => {
    expect(resolveApiKeyStatus({
      ...baseKey,
      disabled: true,
      expiresAt: '2026-08-01T00:00:00Z',
    }, now)).toBe('expired')
  })
})
