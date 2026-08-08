import { describe, expect, it } from 'vitest'
import { isReleaseVersion } from './Step2DeployOptions'

describe('isReleaseVersion', () => {
  it('accepts release tags and rejects source-build versions', () => {
    expect(isReleaseVersion('v2.4.0')).toBe(true)
    expect(isReleaseVersion('2.4.0-rc.1')).toBe(true)
    expect(isReleaseVersion('dev')).toBe(false)
    expect(isReleaseVersion('00151e4')).toBe(false)
    expect(isReleaseVersion(null)).toBe(false)
  })
})
