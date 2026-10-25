import { sha256 } from '@noble/hashes/sha2.js'
import { bytesToHex } from '@noble/hashes/utils.js'

import {
  CHUNK_SIZE,
  HIGH_WATER,
  LOW_WATER,
  encodeControl,
  parseControl,
} from './protocol'

export interface FileSenderHandlers {
  onAck(bytes: number): void
  onDone(): void
  onError(code: string): void
}

export class FileSender {
  private readonly channel: RTCDataChannel
  private readonly file: File
  private readonly handlers: FileSenderHandlers

  private started = false
  // Set once the transfer ends for any reason: completed, failed, or cancelled.
  private stopped = false

  constructor(channel: RTCDataChannel, file: File, handlers: FileSenderHandlers) {
    this.channel = channel
    this.file = file
    this.handlers = handlers
  }

  start(): void {
    if (this.started) return
    this.started = true

    this.channel.binaryType = 'arraybuffer'
    this.channel.bufferedAmountLowThreshold = LOW_WATER
    this.channel.addEventListener('message', this.handleMessage)
    this.channel.addEventListener('close', this.handleClose)
    this.channel.addEventListener('error', this.handleClose)

    // Metadata goes first so the receiver can check the size before chunks arrive.
    this.sendControl({
      type: 'meta',
      name: this.file.name,
      size: this.file.size,
      mime: this.file.type || 'application/octet-stream',
      chunkSize: CHUNK_SIZE,
    })

    void this.pump()
  }

  abort(): void {
    this.stopped = true
    this.detach()
  }

  private handleMessage = (event: MessageEvent): void => {
    if (typeof event.data !== 'string') return
    const message = parseControl(event.data)
    if (!message) return
    if (message.type === 'ack') {
      // An ack can briefly overshoot the file size, and progress must not exceed 100%.
      this.handlers.onAck(Math.min(message.bytes, this.file.size))
      return
    }
    if (message.type === 'done') {
      if (message.ok) {
        this.stopped = true
        this.detach()
        this.handlers.onDone()
      } else {
        this.fail(message.error === 'hash_mismatch' ? 'hash_mismatch' : 'save_failed')
      }
    }
  }

  private handleClose = (): void => {
    if (!this.stopped) this.fail('connection_lost')
  }

  private async pump(): Promise<void> {
    // Hash while sending so the digest is ready when the last chunk leaves.
    const hasher = sha256.create()
    let offset = 0
    try {
      while (offset < this.file.size) {
        if (this.stopped) return
        if (this.channel.readyState !== 'open') {
          this.fail('connection_lost')
          return
        }
        const end = Math.min(offset + CHUNK_SIZE, this.file.size)
        const buffer = await this.file.slice(offset, end).arrayBuffer()
        // The transfer may have been cancelled or disconnected while reading the slice.
        if (this.stopped) return
        hasher.update(new Uint8Array(buffer))
        await this.waitForDrain()
        if (this.stopped) return
        if (this.channel.readyState !== 'open') {
          this.fail('connection_lost')
          return
        }
        this.channel.send(buffer)
        offset = end
      }
      this.sendControl({ type: 'end', sha256: bytesToHex(hasher.digest()) })
    } catch {
      if (!this.stopped) this.fail('connection_lost')
    }
  }

  private waitForDrain(): Promise<void> {
    if (this.channel.bufferedAmount <= HIGH_WATER) return Promise.resolve()
    return new Promise<void>((resolve) => {
      let released = false
      const release = (): void => {
        if (released) return
        released = true
        this.channel.removeEventListener('bufferedamountlow', release)
        this.channel.removeEventListener('close', release)
        clearTimeout(timer)
        resolve()
      }
      // Low water, a close, or 500 ms all release the pump, so a stalled socket cannot hang it.
      const timer = setTimeout(release, 500)
      this.channel.addEventListener('bufferedamountlow', release)
      this.channel.addEventListener('close', release)
    })
  }

  private sendControl(message: Parameters<typeof encodeControl>[0]): void {
    if (this.channel.readyState === 'open') {
      this.channel.send(encodeControl(message))
    }
  }

  private fail(code: string): void {
    // Late events after the transfer ended must not report a second result.
    if (this.stopped) return
    this.stopped = true
    this.detach()
    this.handlers.onError(code)
  }

  private detach(): void {
    this.channel.removeEventListener('message', this.handleMessage)
    this.channel.removeEventListener('close', this.handleClose)
    this.channel.removeEventListener('error', this.handleClose)
  }
}
