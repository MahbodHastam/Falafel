import { sha256 } from '@noble/hashes/sha2.js'
import { bytesToHex } from '@noble/hashes/utils.js'
import { describe, expect, it } from 'vitest'

import { FileSender } from '@/lib/transfer/sender'
import { CHUNK_SIZE } from '@/lib/transfer/protocol'

type Listener = (event: Event) => void

class FakeDataChannel {
  readyState: RTCDataChannelState = 'open'
  bufferedAmount = 0
  bufferedAmountLowThreshold = 0
  binaryType: BinaryType = 'arraybuffer'

  meta: { type: string; name: string; size: number; mime: string; chunkSize: number } | null = null
  endHash = ''
  chunks: ArrayBuffer[] = []
  receivedBytes = 0

  private listeners = new Map<string, Set<Listener>>()

  addEventListener(type: string, listener: Listener): void {
    const set = this.listeners.get(type) ?? new Set()
    set.add(listener)
    this.listeners.set(type, set)
  }

  removeEventListener(type: string, listener: Listener): void {
    this.listeners.get(type)?.delete(listener)
  }

  send(data: string | ArrayBuffer): void {
    if (typeof data === 'string') {
      const message = JSON.parse(data)
      if (message.type === 'meta') this.meta = message
      if (message.type === 'end') {
        this.endHash = message.sha256
        queueMicrotask(() => {
          this.emit('message', new MessageEvent('message', {
            data: JSON.stringify({ type: 'done', ok: true }),
          }))
        })
      }
      return
    }
    this.chunks.push(data.slice(0))
    this.receivedBytes += data.byteLength
    queueMicrotask(() => {
      this.emit('message', new MessageEvent('message', {
        data: JSON.stringify({ type: 'ack', bytes: this.receivedBytes }),
      }))
    })
  }

  close(): void {
    this.readyState = 'closed'
    this.emit('close', new Event('close'))
  }

  private emit(type: string, event: Event): void {
    for (const listener of this.listeners.get(type) ?? []) listener(event)
  }
}

describe('FileSender', () => {
  it('sends metadata, all chunks, a matching digest and waits for completion', async () => {
    const bytes = new Uint8Array(CHUNK_SIZE * 2 + 512)
    for (let index = 0; index < bytes.length; index += 1) bytes[index] = index % 256
    const file = new File([bytes], 'chickpeas.bin', { type: 'application/octet-stream' })
    const channel = new FakeDataChannel()

    const acks: number[] = []
    const completed = new Promise<void>((resolve, reject) => {
      const sender = new FileSender(channel as unknown as RTCDataChannel, file, {
        onAck: (received) => acks.push(received),
        onDone: resolve,
        onError: reject,
      })
      sender.start()
    })

    await completed

    expect(channel.meta?.name).toBe('chickpeas.bin')
    expect(channel.meta?.size).toBe(bytes.length)
    expect(channel.meta?.chunkSize).toBe(CHUNK_SIZE)
    expect(channel.chunks.every((chunk) => chunk.byteLength <= CHUNK_SIZE)).toBe(true)
    expect(channel.chunks.length).toBe(3)

    const joined = new Uint8Array(bytes.length)
    let offset = 0
    for (const chunk of channel.chunks) {
      joined.set(new Uint8Array(chunk), offset)
      offset += chunk.byteLength
    }
    expect(joined).toEqual(bytes)
    expect(channel.endHash).toBe(bytesToHex(sha256(bytes)))
    expect(acks.at(-1)).toBe(bytes.length)
  })
})
