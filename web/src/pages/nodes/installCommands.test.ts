import { describe, expect, it } from 'vitest'
import { buildAgentDownloadCommand, buildAgentInstallCommand } from './installCommands'

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

  it('prefers embedded script content when available', () => {
    const cmd = buildAgentInstallCommand(
      'https://master.example.com/api/install/abc',
      'https://master.example.com/install/abc',
      'IyEvYmluL3NoCg==',
    )

    expect(cmd).toContain('base64 -d')
    expect(cmd).toContain('base64 -D')
    expect(cmd).toContain("'IyEvYmluL3NoCg=='")
    expect(cmd).not.toContain('https://master.example.com/api/install/abc')
  })
})
