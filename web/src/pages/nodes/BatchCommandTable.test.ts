import { describe, expect, it, vi } from 'vitest'
import type { BatchCommandRow } from './BatchCommandTable'
import { getExportableBatchRows } from './BatchCommandTable'

function row(patch: Partial<BatchCommandRow>): BatchCommandRow {
  return {
    nodeId: 1,
    nodeName: 'prod-a',
    status: 'ready',
    command: 'curl install',
    expiresAt: '2099-01-01T00:00:00Z',
    ...patch,
  }
}

describe('getExportableBatchRows', () => {
  it('excludes failed and expired commands from batch export', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-09T00:00:00Z'))
    const rows = [
      row({ nodeId: 1, nodeName: 'ready', expiresAt: '2026-05-09T00:05:00Z' }),
      row({ nodeId: 2, nodeName: 'failed', status: 'failed', errorMessage: 'failed' }),
      row({ nodeId: 3, nodeName: 'expired', expiresAt: '2026-05-08T23:59:59Z' }),
    ]

    expect(getExportableBatchRows(rows).map((item) => item.nodeName)).toEqual(['ready'])

    vi.useRealTimers()
  })
})
