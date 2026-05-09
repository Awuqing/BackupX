import { describe, expect, it } from 'vitest'
import type { UserInfo } from '../../services/auth'
import { canManageNodes } from './NodesPage'

function user(role: string): UserInfo {
  return {
    id: 1,
    username: role,
    displayName: role,
    role,
  }
}

describe('canManageNodes', () => {
  it('allows only admins to manage deployment operations', () => {
    expect(canManageNodes(user('admin'))).toBe(true)
    expect(canManageNodes(user('operator'))).toBe(false)
    expect(canManageNodes(user('viewer'))).toBe(false)
    expect(canManageNodes(null)).toBe(false)
  })
})
