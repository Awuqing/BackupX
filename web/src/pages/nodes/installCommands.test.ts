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

    expect(cmd).toContain('/tmp/bx-agent-install.sh')
    expect(cmd).toContain("'https://master.example.com/install/abc'")
    expect(cmd).toContain('non-script content')
  })

  it('keeps URL install command as primary even when embedded script is available', () => {
    const cmd = buildAgentInstallCommand(
      'https://master.example.com/api/install/abc',
      'https://master.example.com/install/abc',
      'IyEvYmluL3NoCg==',
    )

    expect(cmd).toContain('https://master.example.com/api/install/abc')
    expect(cmd).toContain('https://master.example.com/install/abc')
    expect(cmd).not.toContain('IyEvYmluL3NoCg==')
  })

  it('builds embedded fallback command explicitly', () => {
    const cmd = buildEmbeddedAgentInstallCommand('IyEvYmluL3NoCg==')

    expect(cmd).toContain('base64 -d')
    expect(cmd).toContain('base64 -D')
    expect(cmd).toContain('BACKUPX_AGENT_INSTALL_V1')
    expect(cmd).toContain("'IyEvYmluL3NoCg=='")
  })
})
