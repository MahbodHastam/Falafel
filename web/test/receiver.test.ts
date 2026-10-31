import { sha256 } from '@noble/hashes/sha2.js'
import { bytesToHex } from '@noble/hashes/utils.js'
import { describe, expect, it } from 'vitest'

import { FileReceiver } from '@/lib/transfer/receiver'
import type { DownloadSink } from '@/lib/transfer/sink'
import { ACK_STEP } from '@/lib/transfer/protocol'
import { FakeChannel } from './fake-channel'

class MemorySink implements DownloadSink {
  chunks: ArrayBuffer[] = []
  closed = false
  aborted = false
  failWrites = false
  // Resolve manually to simulate a slow disk.
  gate: Promise<void> | null = null

  async write(chunk: ArrayBuffer): Promise<void> {
    if (this.gate) await this.gate
    if (this.failWrites) throw new Error('disk full')
    this.chunks.push(chunk)
  }
  async close(): Promise<void> {
    this.closed = true
  }
  async abort(): Promise<void> {
    this.aborted = true
  }
}

function bytesOf(size: number): Uint8Array<ArrayBuffer> {
  const bytes = new Uint8Array(new ArrayBuffer(size))
  for (let i = 0; i < size; i += 1) bytes[i] = (i * 7) % 251
  return bytes
}

function setup(size: number, sink = new MemorySink()) {
  const channel = new FakeChannel()
  const events = { progress: [] as number[], complete: [] as unknown[], errors: [] as string[] }
  const receiver = new FileReceiver(channel as unknown as RTCDataChannel, sink, size, {
    onProgress: (bytes) => events.progress.push(bytes),
    onComplete: (result) => events.complete.push(result),
    onError: (code) => events.errors.push(code),
  })
  receiver.start()
  const push = (data: string | ArrayBuffer): void => {
    channel.emit('message', new MessageEvent('message', { data }))
  }
  return { channel, sink, events, push, receiver }
}

const settle = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, 20))

describe('FileReceiver', () => {
  it('writes chunks in order, verifies the digest and saves the file', async () => {
    const data = bytesOf(1000)
    const { channel, sink, events, push } = setup(data.length)

    push(JSON.stringify({ type: 'meta', name: 'a', size: data.length, mime: 'x', chunkSize: 500 }))
    push(data.slice(0, 500).buffer)
    push(data.slice(500).buffer)
    push(JSON.stringify({ type: 'end', sha256: bytesToHex(sha256(data)) }))
    await settle()

    expect(events.errors).toEqual([])
    expect(events.complete).toHaveLength(1)
    expect(sink.closed).toBe(true)
    expect(sink.aborted).toBe(false)
    const saved = new Uint8Array(sink.chunks.reduce((n, c) => n + c.byteLength, 0))
    let at = 0
    for (const chunk of sink.chunks) {
      saved.set(new Uint8Array(chunk), at)
      at += chunk.byteLength
    }
    expect(saved).toEqual(data)
    expect(channel.strings().at(-1)).toEqual({ type: 'done', ok: true })
  })

  it('refuses a corrupt file without closing the sink', async () => {
    const data = bytesOf(64)
    const { channel, sink, events, push } = setup(data.length)

    push(data.buffer.slice(0))
    push(JSON.stringify({ type: 'end', sha256: '00'.repeat(32) }))
    await settle()

    expect(events.errors).toEqual(['hash_mismatch'])
    expect(sink.closed).toBe(false)
    expect(sink.aborted).toBe(true)
    expect(channel.strings().at(-1)).toMatchObject({ type: 'done', ok: false, error: 'hash_mismatch' })
  })

  it('fails when more bytes arrive than the room promised', async () => {
    const { events, push, sink } = setup(10)
    push(new ArrayBuffer(11))
    await settle()
    expect(events.errors).toEqual(['metadata_mismatch'])
    expect(sink.aborted).toBe(true)
  })

  it('fails when the sender announces a different size', async () => {
    const { events, push } = setup(10)
    push(JSON.stringify({ type: 'meta', name: 'a', size: 999, mime: 'x', chunkSize: 1 }))
    await settle()
    expect(events.errors).toEqual(['metadata_mismatch'])
  })

  it('fails when the file ends short', async () => {
    const data = bytesOf(100)
    const { events, push } = setup(200)
    push(data.buffer.slice(0))
    push(JSON.stringify({ type: 'end', sha256: bytesToHex(sha256(data)) }))
    await settle()
    expect(events.errors).toEqual(['metadata_mismatch'])
  })

  it('reports save_failed when the sink rejects writes', async () => {
    const sink = new MemorySink()
    sink.failWrites = true
    const data = bytesOf(32)
    const { events, push } = setup(data.length, sink)
    push(data.buffer.slice(0))
    push(JSON.stringify({ type: 'end', sha256: bytesToHex(sha256(data)) }))
    await settle()
    expect(events.errors).toEqual(['save_failed'])
  })

  it('reports connection_lost when the channel closes mid-transfer', async () => {
    const { channel, events } = setup(100)
    channel.close()
    expect(events.errors).toEqual(['connection_lost'])
  })

  it('ignores malformed control frames', async () => {
    const { events, push } = setup(10)
    push('not json')
    push(JSON.stringify({ type: 'nope' }))
    await settle()
    expect(events.errors).toEqual([])
  })

  it('acks bytes the disk has written, not bytes merely received', async () => {
    const sink = new MemorySink()
    let release!: () => void
    sink.gate = new Promise<void>((resolve) => {
      release = resolve
    })
    const total = ACK_STEP * 2
    const { channel, push } = setup(total, sink)

    push(new ArrayBuffer(ACK_STEP))
    push(new ArrayBuffer(ACK_STEP))
    await settle()
    expect(channel.strings().filter((m) => m.type === 'ack')).toEqual([])

    release()
    await settle()
    const acks = channel.strings().filter((m) => m.type === 'ack') as Array<{ bytes: number }>
    expect(acks.length).toBeGreaterThan(0)
    expect(acks.at(-1)?.bytes).toBe(total)
  })
})
