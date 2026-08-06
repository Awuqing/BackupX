import { describe, expect, it } from 'vitest'
import type { NodeSummary } from '../../types/nodes'
import { buildSourceServerOptions } from './SourceServerSelector'

function node(id: number, name: string, status: NodeSummary['status'], isLocal = false): NodeSummary {
  return {
    id,
    name,
    status,
    isLocal,
    hostname: '',
    ipAddress: '',
    os: '',
    arch: '',
    agentVersion: '',
    lastSeen: '',
    createdAt: '',
  }
}

describe('buildSourceServerOptions', () => {
  it('keeps Master first and disables offline remote sources', () => {
    const options = buildSourceServerOptions([
      node(1, 'local', 'online', true),
      node(2, 'source-b', 'online'),
      node(3, 'source-c', 'offline'),
    ])

    expect(options).toEqual([
      { label: 'Master 本机', value: 0, disabled: false },
      { label: 'source-b', value: 2, disabled: false },
      { label: 'source-c（离线）', value: 3, disabled: true },
    ])
  })
})
