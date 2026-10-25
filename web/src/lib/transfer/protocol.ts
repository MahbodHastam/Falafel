export const CHUNK_SIZE = 64 * 1024
// Pause sending above HIGH_WATER and resume once the channel drops to LOW_WATER.
export const HIGH_WATER = 4 * 1024 * 1024
export const LOW_WATER = 512 * 1024

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

export type ControlMessage = MetaMessage | AckMessage | EndMessage | DoneMessage

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
