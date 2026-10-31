import { hmac } from '@noble/hashes/hmac.js'
import { sha256 } from '@noble/hashes/sha2.js'
import { bytesToHex, utf8ToBytes } from '@noble/hashes/utils.js'

import { encodeControl, parseControl } from './protocol'

/**
 * The signaling server relays the SDP, so on its own it could swap the DTLS
 * fingerprints and sit between the two browsers. The share link carries a random
 * key in its fragment, which browsers never send to a server. Each side proves it
 * knows that key by MACing the two DTLS fingerprints it sees; if the server
 * substituted either one, the MACs no longer match and the transfer is refused.
 */

export type ChannelRole = 'sender' | 'receiver'

const KEY_BYTES = 16
const KEY_PATTERN = /^[A-Za-z0-9_-]{22,64}$/
const AUTH_TIMEOUT_MS = 15_000

export function generateLinkKey(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(KEY_BYTES))
  return btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')
}

// Reads the key from a URL fragment such as `#k=...`; null when missing or malformed.
export function parseLinkKey(hash: string): string | null {
  const key = new URLSearchParams(hash.replace(/^#/, '')).get('k')
  return key && KEY_PATTERN.test(key) ? key : null
}

// Returns the DTLS fingerprint in an SDP, preferring sha-256, normalised to lower case.
export function dtlsFingerprint(sdp: string): string | null {
  const lines = [...sdp.matchAll(/^a=fingerprint:(\S+) ([0-9A-Fa-f:]+)\s*$/gm)]
  const line = lines.find((match) => match[1].toLowerCase() === 'sha-256') ?? lines[0]
  return line ? `${line[1]} ${line[2]}`.toLowerCase() : null
}

export function channelMac(
  key: string,
  role: ChannelRole,
  senderFingerprint: string,
  receiverFingerprint: string,
): string {
  const input = `falafel/v1|${role}|${senderFingerprint}|${receiverFingerprint}`
  return bytesToHex(hmac(sha256, utf8ToBytes(key), utf8ToBytes(input)))
}

function sameMac(a: string, b: string): boolean {
  if (a.length !== b.length) return false
  let diff = 0
  for (let i = 0; i < a.length; i += 1) diff |= a.charCodeAt(i) ^ b.charCodeAt(i)
  return diff === 0
}

export interface ChannelAuthOptions {
  channel: RTCDataChannel
  key: string
  role: ChannelRole
  senderFingerprint: string
  receiverFingerprint: string
  timeoutMs?: number
}

/**
 * Runs the handshake: the sender speaks first, the receiver verifies and replies,
 * and the sender verifies the reply. Resolves once the peer is verified and rejects
 * on a bad MAC, a timeout, or a closed channel. Call it as soon as the channel
 * exists so no message is missed.
 */
export function authenticateChannel(options: ChannelAuthOptions): Promise<void> {
  const { channel, key, role, senderFingerprint, receiverFingerprint } = options
  const peerRole: ChannelRole = role === 'sender' ? 'receiver' : 'sender'
  const own = channelMac(key, role, senderFingerprint, receiverFingerprint)
  const expected = channelMac(key, peerRole, senderFingerprint, receiverFingerprint)

  return new Promise<void>((resolve, reject) => {
    const send = (): void => {
      if (channel.readyState === 'open') channel.send(encodeControl({ type: 'auth', mac: own }))
    }

    const finish = (error?: Error): void => {
      clearTimeout(timer)
      channel.removeEventListener('message', onMessage)
      channel.removeEventListener('close', onClose)
      channel.removeEventListener('open', onOpen)
      if (error) reject(error)
      else resolve()
    }

    const onMessage = (event: MessageEvent): void => {
      if (typeof event.data !== 'string') return
      const message = parseControl(event.data)
      if (message?.type !== 'auth') return
      if (!sameMac(message.mac, expected)) {
        finish(new Error('auth_mismatch'))
        return
      }
      if (role === 'receiver') send()
      finish()
    }
    const onClose = (): void => finish(new Error('auth_closed'))
    const onOpen = (): void => {
      if (role === 'sender') send()
    }
    const timer = setTimeout(() => finish(new Error('auth_timeout')), options.timeoutMs ?? AUTH_TIMEOUT_MS)

    channel.addEventListener('message', onMessage)
    channel.addEventListener('close', onClose)
    if (channel.readyState === 'open') onOpen()
    else channel.addEventListener('open', onOpen)
  })
}
