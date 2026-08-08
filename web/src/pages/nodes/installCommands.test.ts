import { describe, expect, it } from 'vitest'
import { buildAgentDownloadCommand, buildAgentInstallCommand, buildEmbeddedAgentInstallCommand } from './installCommands'

describe('install command builders', () => {
  it('adds script marker validation and fallback install path', () => {
    const cmd = buildAgentInstallCommand('https://master.example.com/api/install/abc')

    expect(cmd).toContain('BACKUPX_AGENT_INSTALL_V1')
    expect(cmd).toContain("'https://master.example.com/api/install/abc'")
    expect(cmd).toContain("'https://master.example.com/install/abc'")
    expect(cmd).toContain('sh "$tmp"')
  })

  it('uses explicit fallback URL when provided', () => {
    const cmd = buildAgentDownloadCommand(
      'https://master.example.com/api/install/abc',
      'https://master.example.com/install/abc',
    )

    expect(cmd).toContain('mktemp /tmp/bx-agent-install.XXXXXX')
    expect(cmd).toContain("'https://master.example.com/install/abc'")
    expect(cmd).toContain('non-script content')
    expect(cmd).toContain('umask 077')
    expect(cmd).toContain('rm -f "$tmp"')
  })

  it('keeps the one-time URL as the primary install command', () => {
    const cmd = buildAgentInstallCommand(
      'https://master.example.com/api/install/abc',
      'https://master.example.com/install/abc',
    )

    expect(cmd).toContain('https://master.example.com/api/install/abc')
    expect(cmd).toContain('https://master.example.com/install/abc')
  })

  it('binds proxy and private CA settings to installer downloads', () => {
    const cmd = buildAgentInstallCommand(
      'https://master.internal/api/install/abc',
      undefined,
      { proxyUrl: 'socks5h://127.0.0.1:1080', caCertFile: '/etc/backupx-agent/ca.pem' },
    )

    expect(cmd).toContain("--proxy 'socks5h://127.0.0.1:1080'")
    expect(cmd).toContain("--cacert '/etc/backupx-agent/ca.pem'")
  })

  it('builds embedded fallback command explicitly', () => {
    const cmd = buildEmbeddedAgentInstallCommand('IyEvYmluL3NoCg==')

    expect(cmd).toContain('base64 -d')
    expect(cmd).toContain('base64 -D')
    expect(cmd).toContain('BACKUPX_AGENT_INSTALL_V1')
    expect(cmd).toContain("'IyEvYmluL3NoCg=='")
  })
})
