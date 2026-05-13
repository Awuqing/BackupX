import { describe, expect, it } from 'vitest'
import type { UserInfo } from '../../services/auth'
import { canManageNodes, formatQueueAge, getNodeHealthView } from './NodesPage'
import type { NodeSummary } from '../../types/nodes'

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

describe('node diagnostics helpers', () => {
  it('formats queue age and health status from backend summaries', () => {
    const node: NodeSummary = {
      id: 1,
      name: 'edge-a',
      hostname: '',
      ipAddress: '',
      status: 'online',
      isLocal: false,
      os: 'linux',
      arch: 'amd64',
      agentVersion: 'v1',
      lastSeen: '2026-05-12T00:00:00Z',
      createdAt: '2026-05-12T00:00:00Z',
      health: 'degraded',
      lastError: 'agent timeout',
      runningTasks: 1,
      queue: {
        pending: 2,
        dispatched: 1,
        depth: 3,
        timeouts: 1,
        oldestActiveAgeSeconds: 125,
      },
    }

    expect(formatQueueAge(node.queue?.oldestActiveAgeSeconds)).toBe('2m')
    expect(getNodeHealthView(node)).toEqual({
      text: '异常',
      badgeStatus: 'warning',
      tagColor: 'orangered',
      tooltip: 'agent timeout',
    })
  })

  it('treats offline nodes as offline even without queue errors', () => {
    const node = {
      id: 2,
      name: 'edge-b',
      hostname: '',
      ipAddress: '',
      status: 'offline',
      isLocal: false,
      os: '',
      arch: '',
      agentVersion: '',
      lastSeen: '',
      createdAt: '',
    } satisfies NodeSummary

    expect(formatQueueAge(0)).toBe('-')
    expect(getNodeHealthView(node).text).toBe('离线')
  })
})
