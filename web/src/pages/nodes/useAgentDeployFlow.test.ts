import { describe, expect, it } from 'vitest'
import type { InstallTokenInput, InstallTokenResult } from '../../types/nodes'
import { createAgentDeployFlow } from './useAgentDeployFlow'

function deployOptions(): InstallTokenInput {
  return {
    mode: 'systemd',
    arch: 'auto',
    agentVersion: 'v2.3.1',
    downloadSrc: 'github',
    ttlSeconds: 900,
  }
}

function tokenResult(overrides: Partial<InstallTokenResult> = {}): InstallTokenResult {
  return {
    installToken: 'install-token',
    expiresAt: '2099-01-01T00:00:00Z',
    url: 'https://master.example.com/api/install/install-token',
    fallbackUrl: 'https://master.example.com/install/install-token',
    scriptBase64: 'IyEvYmluL3NoCg==',
    composeUrl: '',
    fallbackComposeUrl: '',
    ...overrides,
  }
}

describe('createAgentDeployFlow', () => {
  it('creates one node then issues one install token', async () => {
    const calls: string[] = []
    const flow = createAgentDeployFlow({
      batchCreateNodes: async (names) => {
        calls.push(`batch:${names.join(',')}`)
        return [{ id: 7, name: names[0] }]
      },
      createInstallToken: async (nodeId) => {
        calls.push(`token:${nodeId}`)
        return tokenResult()
      },
    })

    const result = await flow.submitNewNodes(['prod-a'], deployOptions())

    expect(calls).toEqual(['batch:prod-a', 'token:7'])
    expect(result.status).toBe('ready')
    expect(result.rows).toHaveLength(1)
    expect(result.rows[0]).toMatchObject({
      nodeId: 7,
      nodeName: 'prod-a',
      status: 'ready',
    })
    expect(result.rows[0].command).toContain('/api/install/install-token')
    expect(result.rows[0].embeddedCommand).toContain('IyEvYmluL3NoCg==')
  })

  it('returns partialFailed when one batch token request fails', async () => {
    const flow = createAgentDeployFlow({
      batchCreateNodes: async (names) => names.map((name, index) => ({ id: index + 1, name })),
      createInstallToken: async (nodeId) => {
        if (nodeId === 2) {
          throw new Error('token service unavailable')
        }
        return tokenResult({ installToken: `tok-${nodeId}`, url: `https://master.example.com/api/install/tok-${nodeId}` })
      },
    })

    const result = await flow.submitNewNodes(['prod-a', 'prod-b', 'prod-c'], deployOptions())

    expect(result.status).toBe('partialFailed')
    expect(result.rows.map((row) => row.status)).toEqual(['ready', 'failed', 'ready'])
    expect(result.rows[1]).toMatchObject({
      nodeId: 2,
      nodeName: 'prod-b',
      status: 'failed',
      errorMessage: 'token service unavailable',
    })
  })

  it('rejects duplicate names before creating nodes', async () => {
    const flow = createAgentDeployFlow({
      batchCreateNodes: async () => {
        throw new Error('should not call batchCreateNodes')
      },
      createInstallToken: async () => tokenResult(),
    })

    await expect(flow.submitNewNodes(['prod-a', ' prod-a '], deployOptions()))
      .rejects.toThrow('批次内重复节点名')
  })
})
