import { sha256 } from '@noble/hashes/sha2.js'
import { bytesToHex } from '@noble/hashes/utils.js'

import {
  ACK_WINDOW,
  CHUNK_SIZE,
  HIGH_WATER,
  LOW_WATER,
  READ_BLOCK_SIZE,
  encodeControl,
  parseControl,
} from './protocol'

export interface FileSenderHandlers {
  onAck(bytes: number): void
  onDone(): void
  onError(code: string): void
}

export interface FileSenderOptions {
  ackWindow?: number
}

export class FileSender {
  private readonly channel: RTCDataChannel
  private readonly file: File
  private readonly handlers: FileSenderHandlers
  private readonly ackWindow: number

  private started = false
  // Set once the transfer ends for any reason: completed, failed, or cancelled.
  private stopped = false
  private sent = 0
  private acked = 0
  private windowWaiter: (() => void) | null = null

  constructor(
    channel: RTCDataChannel,
    file: File,
    handlers: FileSenderHandlers,
    options: FileSenderOptions = {},
  ) {
    this.channel = channel
    this.file = file
    this.handlers = handlers
    this.ackWindow = options.ackWindow ?? ACK_WINDOW
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
    this.releaseWindow()
  }

  private handleMessage = (event: MessageEvent): void => {
    if (typeof event.data !== 'string') return
    const message = parseControl(event.data)
    if (!message) return
    if (message.type === 'ack') {
      // An ack can briefly overshoot the file size, and progress must not exceed 100%.
      this.acked = Math.max(this.acked, Math.min(message.bytes, this.file.size))
      this.handlers.onAck(this.acked)
      this.releaseWindow()
      return
    }
    if (message.type === 'done') {
      if (message.ok) {
        this.stopped = true
        this.detach()
        this.releaseWindow()
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
    try {
      for (let offset = 0; offset < this.file.size; offset += READ_BLOCK_SIZE) {
        if (this.stopped) return
        const block = await this.file
          .slice(offset, Math.min(offset + READ_BLOCK_SIZE, this.file.size))
          .arrayBuffer()
        // The transfer may have been cancelled or disconnected while reading the block.
        if (this.stopped) return
        hasher.update(new Uint8Array(block))

        for (let at = 0; at < block.byteLength; at += CHUNK_SIZE) {
          await this.waitForDrain()
          await this.waitForWindow()
          if (this.stopped) return
          if (this.channel.readyState !== 'open') {
            this.fail('connection_lost')
            return
          }
          const chunk = block.slice(at, at + CHUNK_SIZE)
          this.channel.send(chunk)
          this.sent += chunk.byteLength
        }
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

  // Waits for the receiver to catch up. Acks, abort, completion and failure all wake it.
  private async waitForWindow(): Promise<void> {
    while (!this.stopped && this.sent - this.acked > this.ackWindow) {
      await new Promise<void>((resolve) => {
        this.windowWaiter = resolve
      })
    }
  }

  private releaseWindow(): void {
    const waiter = this.windowWaiter
    this.windowWaiter = null
    waiter?.()
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
    this.releaseWindow()
    this.handlers.onError(code)
  }

  private detach(): void {
    this.channel.removeEventListener('message', this.handleMessage)
    this.channel.removeEventListener('close', this.handleClose)
    this.channel.removeEventListener('error', this.handleClose)
  }
}
