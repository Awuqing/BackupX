import { describe, expect, it } from 'vitest'
import { validateAgentConnection, type AgentConnectionValue } from './AgentConnectionOptions'

function connection(patch: Partial<AgentConnectionValue> = {}): AgentConnectionValue {
  return {
    connectionMode: 'restricted',
    agentMasterUrl: '',
    proxyUrl: '',
    caCertFile: '',
    ...patch,
  }
}

describe('validateAgentConnection', () => {
  it('accepts direct connectivity without overrides', () => {
    expect(validateAgentConnection(connection({ connectionMode: 'direct' }))).toBe('')
  })

  it('accepts an SSH local-forward URL and SOCKS5 proxy', () => {
    expect(validateAgentConnection(connection({ agentMasterUrl: 'http://127.0.0.1:18340' }))).toBe('')
    expect(validateAgentConnection(connection({ proxyUrl: 'socks5h://127.0.0.1:1080' }))).toBe('')
  })

  it('rejects empty restricted settings and relative CA paths', () => {
    expect(validateAgentConnection(connection())).not.toBe('')
    expect(validateAgentConnection(connection({ caCertFile: 'internal-ca.pem' }))).not.toBe('')
  })

  it('rejects credentials and shell-unsafe values before submission', () => {
    expect(validateAgentConnection(connection({ agentMasterUrl: 'https://user:pass@master.example.com' }))).not.toBe('')
    expect(validateAgentConnection(connection({ proxyUrl: 'http://user:pass@proxy.example.com' }))).not.toBe('')
    expect(validateAgentConnection(connection({ caCertFile: '/etc/pki/internal ca.pem' }))).not.toBe('')
  })
})
