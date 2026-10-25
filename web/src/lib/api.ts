export type RoomStatus =
  | 'waiting'
  | 'paired'
  | 'transferring'
  | 'completed'
  | 'failed'
  | 'closed'
  | 'expired'

export interface PublicRoom {
  token: string
  fileName: string
  fileSize: number
  mimeType: string
  passwordRequired: boolean
  status: RoomStatus
  expiresAt: string
}

export interface RoomEvent {
  type: string
  meta?: Record<string, unknown>
  createdAt: string
}

export interface OwnerRoom extends PublicRoom {
  bytesReceived: number
  failedAttempts: number
  createdAt: string
  connectedAt: string | null
  completedAt: string | null
  closedAt: string | null
  events: RoomEvent[]
}

export interface RoomStats {
  status: RoomStatus
  bytesReceived: number
  failedAttempts: number
  connectedAt: string | null
  completedAt: string | null
  closedAt: string | null
}

export interface AppConfig {
  iceServers: RTCIceServer[]
  maxFileSize: number
  minPasswordLength: number
  roomTTLSeconds: number
  baseURL: string
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly field?: string
  readonly reason?: string
  readonly retryAfter?: number

  constructor(status: number, code: string, details?: { field?: string; reason?: string; retryAfter?: number }) {
    super(code)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.field = details?.field
    this.reason = details?.reason
    this.retryAfter = details?.retryAfter
  }
}

interface ErrorBody {
  error?: { code?: string; field?: string; reason?: string }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, {
      ...init,
      headers: {
        Accept: 'application/json',
        ...(init.body ? { 'Content-Type': 'application/json' } : {}),
        ...(init.headers ?? {}),
      },
    })
  } catch {
    // fetch only throws when the request never reached the server, so there is no status.
    throw new ApiError(0, 'network_error')
  }

  const text = await response.text()
  let body: unknown = null
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = null
    }
  }

  if (!response.ok) {
    const parsed = (body ?? {}) as ErrorBody
    // A missing header parses as 0, which the check below treats as absent.
    const retryAfter = Number(response.headers.get('Retry-After'))
    throw new ApiError(response.status, parsed.error?.code ?? `http_${response.status}`, {
      field: parsed.error?.field,
      reason: parsed.error?.reason,
      retryAfter: Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : undefined,
    })
  }
  return body as T
}

let configPromise: Promise<AppConfig> | null = null

export function getConfig(): Promise<AppConfig> {
  if (!configPromise) {
    configPromise = request<AppConfig>('/api/config').catch((error) => {
      // Drop the failed promise so a later caller can retry instead of replaying the error.
      configPromise = null
      throw error
    })
  }
  return configPromise
}

export interface CreateRoomInput {
  fileName: string
  fileSize: number
  mimeType: string
  password?: string
}

export interface CreatedRoom {
  token: string
  url: string
  ownerSecret: string
  expiresAt: string
  room: PublicRoom
}

export function createRoom(input: CreateRoomInput): Promise<CreatedRoom> {
  return request<CreatedRoom>('/api/rooms', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function getRoom(token: string): Promise<PublicRoom> {
  return request<PublicRoom>(`/api/rooms/${encodeURIComponent(token)}`)
}

export interface JoinResult {
  ticket: string
  expiresAt: string
}

export function joinRoom(token: string, password: string): Promise<JoinResult> {
  return request<JoinResult>(`/api/rooms/${encodeURIComponent(token)}/join`, {
    method: 'POST',
    body: JSON.stringify({ password }),
  })
}

export async function closeRoom(token: string, ownerSecret: string): Promise<void> {
  await request<unknown>(`/api/rooms/${encodeURIComponent(token)}/close`, {
    method: 'POST',
    body: JSON.stringify({ ownerSecret }),
  })
}

export function closeRoomOnUnload(token: string, ownerSecret: string): void {
  // sendBeacon is the only request browsers reliably deliver while the page unloads.
  const payload = new Blob([JSON.stringify({ ownerSecret })], { type: 'application/json' })
  navigator.sendBeacon(`/api/rooms/${encodeURIComponent(token)}/close`, payload)
}
