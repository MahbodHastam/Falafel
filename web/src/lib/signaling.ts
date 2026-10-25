import type { OwnerRoom, PublicRoom, RoomEvent, RoomStats } from './api'

export type SignalingRole = 'owner' | 'downloader'

export interface OwnerAuth {
  role: 'owner'
  ownerSecret: string
}

export interface DownloaderAuth {
  role: 'downloader'
  ticket: string
}

export type SignalingAuth = OwnerAuth | DownloaderAuth

export interface SignalData {
  sdp?: RTCSessionDescriptionInit
  candidate?: RTCIceCandidateInit
}

export interface ReadyEvent {
  type: 'ready'
  role: SignalingRole
  room: OwnerRoom | PublicRoom
  peerOnline: boolean
}

export type SignalingEvent =
  | ReadyEvent
  | { type: 'peer-joined'; peerOnline: boolean }
  | { type: 'peer-left'; reason?: string }
  | { type: 'signal'; from: SignalingRole; data: SignalData }
  | { type: 'stats'; stats: RoomStats; events: RoomEvent[] }
  | { type: 'closed'; reason: string }
  | { type: 'error'; code: string }

export class SignalingError extends Error {
  readonly code: string

  constructor(code: string) {
    super(code)
    this.name = 'SignalingError'
    this.code = code
  }
}

const CLOSE_CODES: Record<number, string> = {
  4400: 'bad_request',
  4401: 'unauthorized',
  4404: 'not_found',
  4409: 'room_busy',
}

interface PendingReady {
  resolve: (event: ReadyEvent) => void
  reject: (error: Error) => void
}

export class SignalingClient {
  private readonly token: string
  private readonly auth: SignalingAuth
  private socket: WebSocket | null = null
  private pending: PendingReady | null = null
  private listeners = new Set<(event: SignalingEvent) => void>()
  // Set when we close on purpose or have already reported a close, so `closed` fires once.
  private closed = false

  constructor(token: string, auth: SignalingAuth) {
    this.token = token
    this.auth = auth
  }

  connect(): Promise<ReadyEvent> {
    if (typeof WebSocket === 'undefined') {
      return Promise.reject(new SignalingError('unsupported'))
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const socket = new WebSocket(
      `${protocol}//${window.location.host}/api/ws/${encodeURIComponent(this.token)}`,
    )
    this.socket = socket

    return new Promise<ReadyEvent>((resolve, reject) => {
      this.pending = { resolve, reject }

      socket.addEventListener('open', () => {
        this.send({ type: 'auth', ...this.auth })
      })

      socket.addEventListener('message', (event) => {
        this.handleMessage(event.data)
      })

      socket.addEventListener('error', () => {
        this.rejectPending('connection_lost')
      })

      socket.addEventListener('close', (event) => {
        // Closing before `ready` means connect() can never resolve.
        this.rejectPending(CLOSE_CODES[event.code] ?? 'connection_lost')
        if (!this.closed) {
          this.closed = true
          this.emit({ type: 'closed', reason: event.reason || 'socket_closed' })
        }
      })
    })
  }

  onEvent(listener: (event: SignalingEvent) => void): () => void {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  sendSignal(data: SignalData): void {
    this.send({ type: 'signal', data })
  }

  sendStatus(phase: 'transferring' | 'completed' | 'failed', bytes: number): void {
    this.send({ type: 'status', phase, bytes })
  }

  close(): void {
    this.closed = true
    this.listeners.clear()
    if (this.socket && this.socket.readyState <= WebSocket.OPEN) {
      this.socket.close(1000, 'client_closed')
    }
    this.socket = null
  }

  private send(payload: Record<string, unknown>): void {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify(payload))
    }
  }

  private rejectPending(code: string): void {
    // `pending` is cleared as soon as `ready` arrives, so this only affects connect().
    const pending = this.pending
    this.pending = null
    pending?.reject(new SignalingError(code))
  }

  private handleMessage(raw: unknown): void {
    // A frame that is not a JSON object with a string type is ignored, not fatal.
    if (typeof raw !== 'string') return
    let message: Record<string, unknown>
    try {
      message = JSON.parse(raw)
    } catch {
      return
    }
    const type = message.type
    if (typeof type !== 'string') return

    if (type === 'ready') {
      const event: ReadyEvent = {
        type: 'ready',
        role: message.role === 'owner' ? 'owner' : 'downloader',
        room: message.room as OwnerRoom | PublicRoom,
        peerOnline: Boolean(message.peerOnline),
      }
      const pending = this.pending
      this.pending = null
      pending?.resolve(event)
      this.emit(event)
      return
    }

    if (type === 'error') {
      const code = typeof message.code === 'string' ? message.code : 'generic'
      this.emit({ type: 'error', code })
      this.rejectPending(code)
      return
    }

    switch (type) {
      case 'peer-joined':
        this.emit({ type: 'peer-joined', peerOnline: true })
        break
      case 'peer-left':
        this.emit({ type: 'peer-left', reason: String(message.reason ?? '') })
        break
      case 'signal':
        this.emit({
          type: 'signal',
          from: message.from === 'owner' ? 'owner' : 'downloader',
          data: (message.data ?? {}) as SignalData,
        })
        break
      case 'stats':
        this.emit({
          type: 'stats',
          stats: message.stats as RoomStats,
          events: (message.events ?? []) as RoomEvent[],
        })
        break
      case 'closed':
        this.closed = true
        this.emit({ type: 'closed', reason: String(message.reason ?? 'closed') })
        break
    }
  }

  private emit(event: SignalingEvent): void {
    for (const listener of this.listeners) {
      listener(event)
    }
  }
}
