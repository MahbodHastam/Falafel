import { describe, expect, it } from 'vitest'

import {
  authenticateChannel,
  channelMac,
  dtlsFingerprint,
  generateLinkKey,
  parseLinkKey,
} from '@/lib/transfer/channel-auth'
import { channelPair } from './fake-channel'

const SENDER_FP = 'sha-256 aa:bb:cc'
const RECEIVER_FP = 'sha-256 11:22:33'

function run(
  channels: ReturnType<typeof channelPair>,
  senderKey: string,
  receiverKey: string,
  receiverSees = { sender: SENDER_FP, receiver: RECEIVER_FP },
) {
  const [a, b] = channels
  const sender = authenticateChannel({
    channel: a as unknown as RTCDataChannel,
    key: senderKey,
    role: 'sender',
    senderFingerprint: SENDER_FP,
    receiverFingerprint: RECEIVER_FP,
    timeoutMs: 200,
  })
  const receiver = authenticateChannel({
    channel: b as unknown as RTCDataChannel,
    key: receiverKey,
    role: 'receiver',
    senderFingerprint: receiverSees.sender,
    receiverFingerprint: receiverSees.receiver,
    timeoutMs: 200,
  })
  return { sender, receiver }
}

describe('link key', () => {
  it('generates url-safe keys that parse back from a fragment', () => {
    const key = generateLinkKey()
    expect(key).toMatch(/^[A-Za-z0-9_-]{22}$/)
    expect(parseLinkKey(`#k=${key}`)).toBe(key)
    expect(generateLinkKey()).not.toBe(key)
  })

  it('rejects a missing or malformed key', () => {
    expect(parseLinkKey('')).toBeNull()
    expect(parseLinkKey('#')).toBeNull()
    expect(parseLinkKey('#k=short')).toBeNull()
    expect(parseLinkKey('#k=' + 'a'.repeat(22) + '!')).toBeNull()
    expect(parseLinkKey('#other=' + 'a'.repeat(22))).toBeNull()
  })
})

describe('dtlsFingerprint', () => {
  it('prefers sha-256 and normalises case', () => {
    const sdp = [
      'v=0',
      'a=fingerprint:sha-512 AA:BB',
      'a=fingerprint:SHA-256 0A:1B:2C',
      'a=setup:actpass',
    ].join('\r\n')
    expect(dtlsFingerprint(sdp)).toBe('sha-256 0a:1b:2c')
  })

  it('returns null without a fingerprint', () => {
    expect(dtlsFingerprint('v=0\r\ns=-')).toBeNull()
  })
})

describe('authenticateChannel', () => {
  it('verifies both sides when they share the key and see the same fingerprints', async () => {
    const { sender, receiver } = run(channelPair(), 'k'.repeat(22), 'k'.repeat(22))
    await expect(Promise.all([sender, receiver])).resolves.toBeDefined()
  })

  it('refuses when the receiver does not know the link key', async () => {
    const { sender, receiver } = run(channelPair(), 'k'.repeat(22), 'x'.repeat(22))
    await expect(receiver).rejects.toThrow('auth_mismatch')
    await expect(sender).rejects.toThrow('auth_timeout')
  })

  it('detects a server that swapped a DTLS fingerprint', async () => {
    // The receiver was told a different sender fingerprint than the sender actually has.
    const { sender, receiver } = run(channelPair(), 'k'.repeat(22), 'k'.repeat(22), {
      sender: 'sha-256 de:ad:be:ef',
      receiver: RECEIVER_FP,
    })
    await expect(receiver).rejects.toThrow('auth_mismatch')
    await expect(sender).rejects.toThrow()
  })

  it('does not let a peer replay the sender MAC in the receiver role', () => {
    const key = 'k'.repeat(22)
    expect(channelMac(key, 'sender', SENDER_FP, RECEIVER_FP)).not.toBe(
      channelMac(key, 'receiver', SENDER_FP, RECEIVER_FP),
    )
  })

  it('rejects when the channel closes first', async () => {
    const [a, b] = channelPair()
    const sender = authenticateChannel({
      channel: a as unknown as RTCDataChannel,
      key: 'k'.repeat(22),
      role: 'sender',
      senderFingerprint: SENDER_FP,
      receiverFingerprint: RECEIVER_FP,
    })
    a.close()
    await expect(sender).rejects.toThrow('auth_closed')
    b.close()
  })
})
