import { describe, expect, it } from 'vitest'

import { formatBytes, formatDuration, formatPercent, formatSpeed } from '@/lib/format'

const translate = (key: string, values?: Record<string, unknown>): string => {
  if (key === 'common.seconds') return `${values?.n}s`
  if (key === 'common.minutesSeconds') return `${values?.m}m ${values?.s}s`
  if (key === 'common.lessThanSecond') return 'less than a second'
  return key
}

describe('formatBytes', () => {
  it('scales through binary units', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(1024)).toBe('1 KB')
    expect(formatBytes(1536)).toBe('1.5 KB')
    expect(formatBytes(10 * 1024 * 1024)).toBe('10 MB')
    expect(formatBytes(2.5 * 1024 * 1024 * 1024)).toBe('2.5 GB')
  })

  it('uses Persian digits for the Persian locale', () => {
    const formatted = formatBytes(1536, 'fa')
    expect(formatted).toContain('KB')
    expect(formatted).toMatch(/[\u06F0-\u06F9]/)
  })
})

describe('formatDuration', () => {
  it('uses a human phrase below one second', () => {
    expect(formatDuration(300, 'en', translate)).toBe('less than a second')
  })

  it('formats seconds and minutes', () => {
    expect(formatDuration(5_000, 'en', translate)).toBe('5s')
    expect(formatDuration(95_000, 'en', translate)).toBe('1m 35s')
  })
})

describe('formatPercent and formatSpeed', () => {
  it('clamps and rounds probabilities', () => {
    expect(formatPercent(0.425)).toBe('43%')
    expect(formatPercent(1.4)).toBe('100%')
    expect(formatPercent(-1)).toBe('0%')
  })

  it('appends a per-second suffix', () => {
    expect(formatSpeed(1024)).toBe('1 KB/s')
  })
})
