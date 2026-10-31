export const CHUNK_SIZE = 64 * 1024
// Pause sending above HIGH_WATER and resume once the channel drops to LOW_WATER.
export const HIGH_WATER = 4 * 1024 * 1024
export const LOW_WATER = 512 * 1024
// The sender reads the file in blocks this size, then cuts them into CHUNK_SIZE messages.
export const READ_BLOCK_SIZE = 1024 * 1024
// The sender stops once this many bytes are unacknowledged, so a slow disk on the
// receiving side cannot make the receiver queue the whole file in memory.
export const ACK_WINDOW = 32 * 1024 * 1024
// The receiver acknowledges written bytes at least this often (and every ACK_INTERVAL_MS).
export const ACK_STEP = 1024 * 1024
export const ACK_INTERVAL_MS = 250

export interface MetaMessage {
  type: 'meta'
  name: string
  size: number
  mime: string
  chunkSize: number
}

export interface AckMessage {
  type: 'ack'
  bytes: number
}

export interface EndMessage {
  type: 'end'
  sha256: string
}

export interface DoneMessage {
  type: 'done'
  ok: boolean
  error?: string
}

// Proves the peer holds the link's secret and sees the same DTLS fingerprints. See channel-auth.ts.
export interface AuthMessage {
  type: 'auth'
  mac: string
}

export type ControlMessage = MetaMessage | AckMessage | EndMessage | DoneMessage | AuthMessage

// Malformed frames return null instead of throwing, so one bad peer cannot break the transfer.
export function parseControl(raw: string): ControlMessage | null {
  let value: unknown
  try {
    value = JSON.parse(raw)
  } catch {
    return null
  }
  if (typeof value !== 'object' || value === null) return null
  const message = value as Record<string, unknown>
  switch (message.type) {
    case 'meta':
      if (
        typeof message.name !== 'string' ||
        typeof message.size !== 'number' ||
        typeof message.mime !== 'string' ||
        typeof message.chunkSize !== 'number'
      ) {
        return null
      }
      return {
        type: 'meta',
        name: message.name,
        size: message.size,
        mime: message.mime,
        chunkSize: message.chunkSize,
      }
    case 'ack':
      if (typeof message.bytes !== 'number') return null
      return { type: 'ack', bytes: message.bytes }
    case 'end':
      if (typeof message.sha256 !== 'string') return null
      return { type: 'end', sha256: message.sha256 }
    case 'auth':
      if (typeof message.mac !== 'string') return null
      return { type: 'auth', mac: message.mac }
    case 'done':
      return {
        type: 'done',
        ok: Boolean(message.ok),
        error: typeof message.error === 'string' ? message.error : undefined,
      }
    default:
      return null
  }
}

export function encodeControl(message: ControlMessage): string {
  return JSON.stringify(message)
}
