import { describe, expect, it } from 'vitest'

import en from '@/i18n/en'
import fa from '@/i18n/fa'

type Messages = Record<string, unknown>

function flatten(messages: Messages, prefix = ''): Map<string, string> {
  const out = new Map<string, string>()
  for (const [key, value] of Object.entries(messages)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') {
      out.set(path, value)
    } else if (value && typeof value === 'object') {
      for (const [nested, nestedValue] of flatten(value as Messages, path)) {
        out.set(nested, nestedValue)
      }
    }
  }
  return out
}

function placeholders(message: string): string[] {
  return [...message.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort()
}

describe('translations', () => {
  const english = flatten(en as Messages)
  const persian = flatten(fa as Messages)

  it('has exactly the same keys in both languages', () => {
    expect([...persian.keys()].sort()).toEqual([...english.keys()].sort())
  })

  it('keeps interpolation placeholders in sync', () => {
    for (const [key, englishMessage] of english) {
      const persianMessage = persian.get(key)
      expect(persianMessage, `missing Persian translation for ${key}`).toBeTruthy()
      expect(placeholders(persianMessage as string), `placeholder mismatch at ${key}`).toEqual(
        placeholders(englishMessage),
      )
    }
  })

  it('uses plain punctuation everywhere', () => {
    const forbidden = /[—–…·’‘“”]/
    for (const [key, message] of [...english, ...persian]) {
      expect(message.match(forbidden), `unusual character at ${key}: ${message}`).toBeNull()
    }
  })

  it('does not leave English copy in the Persian bundle', () => {
    const allowed = new Set(['WebRTC', 'SHA-256', 'Go', 'GitHub', 'EN', 'FA'])
    for (const [key, message] of persian) {
      if (/^[A-Za-z0-9 :·—.,%()/-]+$/.test(message) && !allowed.has(message.trim())) {
        throw new Error(`Persian message at ${key} looks English: ${message}`)
      }
    }
  })
})
