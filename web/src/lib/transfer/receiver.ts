import { sha256 } from '@noble/hashes/sha2.js'
import { bytesToHex } from '@noble/hashes/utils.js'

import type { DownloadSink } from './sink'
import { ACK_INTERVAL_MS, ACK_STEP, encodeControl, parseControl } from './protocol'

export interface FileReceiverHandlers {
  onProgress(bytes: number, total: number): void
  onComplete(result: { sha256: string; size: number }): void
  onError(code: string): void
}

export class FileReceiver {
  private readonly channel: RTCDataChannel
  private readonly sink: DownloadSink
  private readonly expectedSize: number
  private readonly handlers: FileReceiverHandlers

  private hasher = sha256.create()
  private bytes = 0
  // Bytes the sink has finished writing; acks report this, not bytes received,
  // so the sender slows down when the disk is the bottleneck.
  private written = 0
  private ackedBytes = 0
  private lastAckAt = 0
  private writeChain: Promise<void> = Promise.resolve()
  private sinkFailed = false
  private finished = false
  private started = false

  constructor(
    channel: RTCDataChannel,
    sink: DownloadSink,
    expectedSize: number,
    handlers: FileReceiverHandlers,
  ) {
    this.channel = channel
    this.sink = sink
    this.expectedSize = expectedSize
    this.handlers = handlers
  }

  start(): void {
    if (this.started) return
    this.started = true
    this.channel.binaryType = 'arraybuffer'
    this.channel.addEventListener('message', this.handleMessage)
    this.channel.addEventListener('close', this.handleClose)
    this.channel.addEventListener('error', this.handleClose)
  }

  abort(): void {
    if (this.finished) return
    this.finished = true
    this.sendDone(false, 'cancelled')
    this.detach()
    void this.sink.abort()
  }

  private handleMessage = (event: MessageEvent): void => {
    if (typeof event.data === 'string') {
      this.handleControl(event.data)
      return
    }
    if (event.data instanceof ArrayBuffer) {
      this.handleChunk(event.data)
      return
    }
    // Some browsers deliver binary frames as Blob even with binaryType set.
    if (event.data instanceof Blob) {
      void event.data.arrayBuffer().then((buffer) => this.handleChunk(buffer))
    }
  }

  private handleControl(raw: string): void {
    const message = parseControl(raw)
    if (!message || this.finished) return
    if (message.type === 'meta') {
      if (message.size !== this.expectedSize) {
        this.fail('metadata_mismatch')
      }
      return
    }
    if (message.type === 'end') {
      void this.finish(message.sha256)
    }
  }

  private handleChunk(buffer: ArrayBuffer): void {
    if (this.finished) return
    if (this.bytes + buffer.byteLength > this.expectedSize) {
      this.fail('metadata_mismatch')
      return
    }
    this.bytes += buffer.byteLength
    this.hasher.update(new Uint8Array(buffer))
    // Sink writes run one at a time, so a slow disk cannot reorder chunks.
    this.writeChain = this.writeChain
      .then(async () => {
        await this.sink.write(buffer)
        this.written += buffer.byteLength
        this.sendAck(false)
      })
      .catch(() => {
        this.sinkFailed = true
      })
    this.handlers.onProgress(this.bytes, this.expectedSize)
  }

  private async finish(remoteHash: string): Promise<void> {
    if (this.finished) return
    this.finished = true
    this.detach()

    await this.writeChain.catch(() => undefined)

    if (this.sinkFailed) {
      this.reportFailure('save_failed')
      return
    }

    // Check size and hash before closing the sink, so a corrupt file is never saved.
    if (this.bytes !== this.expectedSize || this.bytes === 0) {
      this.reportFailure('metadata_mismatch')
      return
    }

    const digest = bytesToHex(this.hasher.digest())
    if (digest !== remoteHash.toLowerCase()) {
      this.reportFailure('hash_mismatch')
      return
    }

    try {
      await this.sink.close()
    } catch {
      // close() failed partway, so report save_failed without touching the sink again.
      this.sendDone(false, 'save_failed')
      this.handlers.onError('save_failed')
      return
    }

    this.written = this.bytes
    this.sendAck(true)
    this.sendDone(true)
    this.handlers.onComplete({ sha256: digest, size: this.bytes })
  }

  private fail(code: string): void {
    if (this.finished) return
    this.finished = true
    this.detach()
    this.reportFailure(code)
  }

  private reportFailure(code: string): void {
    this.sendDone(false, code)
    this.handlers.onError(code)
    void this.sink.abort()
  }

  private handleClose = (): void => {
    if (!this.finished) this.fail('connection_lost')
  }

  private sendAck(force: boolean): void {
    const now = Date.now()
    // Ack every ACK_STEP bytes or ACK_INTERVAL_MS, whichever comes first, so a fast
    // transfer does not flood the control channel; `force` skips the throttle.
    const due =
      this.written - this.ackedBytes >= ACK_STEP ||
      (this.written > this.ackedBytes && now - this.lastAckAt >= ACK_INTERVAL_MS)
    if (!force && !due) return
    this.lastAckAt = now
    this.ackedBytes = this.written
    if (this.channel.readyState === 'open') {
      this.channel.send(encodeControl({ type: 'ack', bytes: this.written }))
    }
  }

  private sendDone(ok: boolean, error?: string): void {
    if (this.channel.readyState !== 'open') return
    this.channel.send(encodeControl({ type: 'done', ok, error }))
  }

  private detach(): void {
    this.channel.removeEventListener('message', this.handleMessage)
    this.channel.removeEventListener('close', this.handleClose)
    this.channel.removeEventListener('error', this.handleClose)
  }
}
