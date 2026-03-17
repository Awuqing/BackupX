import { describe, expect, it } from 'vitest'
import { formatBytes, formatDuration, formatPercent } from './format'

describe('format utils', () => {
  it('formats bytes into readable units', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1024)).toBe('1 KB')
    expect(formatBytes(1536)).toBe('1.5 KB')
  })

  it('formats percent and duration', () => {
    expect(formatPercent(0.56)).toBe('56%')
    expect(formatDuration(45)).toBe('45 秒')
    expect(formatDuration(3661)).toBe('1 小时 1 分 1 秒')
  })
})
