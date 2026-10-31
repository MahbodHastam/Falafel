import { describe, expect, it } from 'vitest'

import { sanitizeFileName } from '@/lib/transfer/sink'

describe('sanitizeFileName', () => {
  it('keeps ordinary names, including non-latin ones', () => {
    expect(sanitizeFileName('chickpeas.tar.gz')).toBe('chickpeas.tar.gz')
    expect(sanitizeFileName('گزارش نهایی.pdf')).toBe('گزارش نهایی.pdf')
  })

  it('removes path separators and shell-hostile characters', () => {
    expect(sanitizeFileName('../../etc/passwd')).toBe('.._.._etc_passwd')
    expect(sanitizeFileName('a:b*c?"d<e>f|g')).toBe('a_b_c__d_e_f_g')
  })

  it('strips control characters and direction overrides', () => {
    expect(sanitizeFileName('a\u0000b\u001fc')).toBe('abc')
    expect(sanitizeFileName('invoice‮fdp.exe')).toBe('invoicefdp.exe')
    expect(sanitizeFileName('x⁦y⁩.txt')).toBe('xy.txt')
  })

  it('keeps ZWNJ, which Persian words need', () => {
    expect(sanitizeFileName('می‌خواهم.txt')).toBe('می‌خواهم.txt')
  })

  it('defuses Windows reserved names and trailing dots', () => {
    expect(sanitizeFileName('CON')).toBe('_CON')
    expect(sanitizeFileName('nul.txt')).toBe('_nul.txt')
    expect(sanitizeFileName('report. . ')).toBe('report')
    expect(sanitizeFileName('console.log')).toBe('console.log')
  })

  it('bounds the length and keeps the extension', () => {
    const name = sanitizeFileName('a'.repeat(500) + '.mp4')
    expect(name.length).toBe(200)
    expect(name.endsWith('.mp4')).toBe(true)
  })

  it('falls back when nothing usable is left', () => {
    expect(sanitizeFileName('')).toBe('falafel-file')
    expect(sanitizeFileName('\u0000‮')).toBe('falafel-file')
    expect(sanitizeFileName('...')).toBe('falafel-file')
  })
})
