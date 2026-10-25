import { computed, onBeforeUnmount, ref } from 'vue'

import {
  ApiError,
  closeRoom,
  closeRoomOnUnload,
  createRoom,
  getConfig,
  type AppConfig,
  type OwnerRoom,
  type RoomEvent,
  type RoomStats,
} from '@/lib/api'
import { PeerLink } from '@/lib/peer'
import {
  SignalingClient,
  SignalingError,
  type SignalingEvent,
  type SignalData,
} from '@/lib/signaling'
import { FileSender } from '@/lib/transfer/sender'

export type SendPhase =
  | 'idle'
  | 'creating'
  | 'waiting'
  | 'connecting'
  | 'transferring'
  | 'completed'
  | 'failed'
  | 'closed'

// The receiver's last acks can arrive after its socket closes, so a dropped
// peer during a transfer gets a short grace period before we call it failed.
const PEER_LEFT_GRACE_MS = 1200

/**
 * Drives the sender side: create a room, keep the link alive, and stream the
 * file once the receiver joins.
 */
export function useSendSession() {
  const phase = ref<SendPhase>('idle')
  const room = ref<OwnerRoom | null>(null)
  const shareURL = ref('')
  const peerOnline = ref(false)
  const receivedBytes = ref(0)
  const errorCode = ref('')
  const config = ref<AppConfig | null>(null)

  let token = ''
  let ownerSecret = ''
  let signaling: SignalingClient | null = null
  let peer: PeerLink | null = null
  let channel: RTCDataChannel | null = null
  let sender: FileSender | null = null
  let file: File | null = null
  let lastStatusAt = 0
  let unmounted = false
  let peerLeftTimer: number | undefined

  const fileSize = computed(() => room.value?.fileSize ?? 0)
  const progress = computed(() =>
    fileSize.value > 0 ? Math.min(1, receivedBytes.value / fileSize.value) : 0,
  )
  const active = computed(() => Boolean(room.value))

  async function start(selected: File, password: string): Promise<void> {
    if (phase.value === 'creating') return
    resetPeer()
    errorCode.value = ''
    phase.value = 'creating'
    file = selected

    if (typeof RTCPeerConnection === 'undefined') {
      errorCode.value = 'unsupported'
      phase.value = 'failed'
      return
    }

    try {
      config.value = await getConfig()
      if (selected.size > config.value.maxFileSize) {
        errorCode.value = 'file_too_large'
        phase.value = 'failed'
        return
      }

      const created = await createRoom({
        fileName: selected.name,
        fileSize: selected.size,
        mimeType: selected.type || 'application/octet-stream',
        password: password || undefined,
      })
      if (unmounted) return

      token = created.token
      ownerSecret = created.ownerSecret
      shareURL.value = created.url
      peerOnline.value = false
      receivedBytes.value = 0
      room.value = {
        ...created.room,
        bytesReceived: 0,
        failedAttempts: 0,
        createdAt: new Date().toISOString(),
        connectedAt: null,
        completedAt: null,
        closedAt: null,
        events: [{ type: 'created', createdAt: new Date().toISOString() }],
      }
      phase.value = 'waiting'
      openOwnerSocket()
    } catch (error) {
      if (unmounted) return
      errorCode.value = mapError(error)
      phase.value = 'failed'
    }
  }

  function openOwnerSocket(): void {
    if (!token || !ownerSecret) return
    signaling = new SignalingClient(token, { role: 'owner', ownerSecret })
    signaling.onEvent(handleOwnerEvent)
    void signaling.connect().catch((error) => {
      if (unmounted) return
      errorCode.value = mapError(error)
      phase.value = 'failed'
    })
  }

  function handleOwnerEvent(event: SignalingEvent): void {
    if (unmounted) return
    switch (event.type) {
      case 'ready': {
        const snapshot = event.room as OwnerRoom
        room.value = { ...snapshot, events: snapshot.events ?? [] }
        receivedBytes.value = snapshot.bytesReceived
        phase.value = snapshot.status === 'completed' ? 'completed' : 'waiting'
        peerOnline.value = event.peerOnline
        if (event.peerOnline) void connectPeer()
        break
      }
      case 'peer-joined':
        window.clearTimeout(peerLeftTimer)
        peerOnline.value = true
        errorCode.value = ''
        if (phase.value === 'waiting' || phase.value === 'failed') {
          phase.value = 'connecting'
        }
        appendEvent('downloader_joined')
        void connectPeer()
        break
      case 'peer-left':
        peerOnline.value = false
        handlePeerLeft()
        break
      case 'signal':
        void applySignal(event.data)
        break
      case 'stats':
        if (event.stats.status === 'completed') {
          window.clearTimeout(peerLeftTimer)
          errorCode.value = ''
        }
        mergeStats(event.stats, event.events)
        break
      case 'closed':
        peerOnline.value = false
        resetPeer()
        if (phase.value !== 'completed') phase.value = 'closed'
        break
      case 'error':
        errorCode.value = event.code
        phase.value = 'failed'
        break
    }
  }

  function handlePeerLeft(): void {
    if (phase.value === 'completed' || phase.value === 'closed') return

    if (phase.value === 'transferring') {
      window.clearTimeout(peerLeftTimer)
      peerLeftTimer = window.setTimeout(() => {
        if (phase.value !== 'transferring') return
        resetPeer()
        phase.value = 'failed'
        errorCode.value = 'peer_left'
        appendEvent('transfer_failed')
      }, PEER_LEFT_GRACE_MS)
      return
    }

    resetPeer()
    if (phase.value === 'connecting') {
      phase.value = 'failed'
      errorCode.value = 'peer_left'
      appendEvent('transfer_failed')
    }
  }

  async function connectPeer(): Promise<void> {
    if (!file) return
    resetPeer()

    const connection = new PeerLink({
      iceServers: config.value?.iceServers ?? [],
      sendSignal: (data) => signaling?.sendSignal(data),
      onFailed: () => {
        if (phase.value === 'completed') return
        errorCode.value = 'ice_failed'
        phase.value = 'failed'
        signaling?.sendStatus('failed', receivedBytes.value)
      },
    })
    peer = connection
    phase.value = 'connecting'

    const dataChannel = connection.connection.createDataChannel('falafel', { ordered: true })
    channel = dataChannel

    dataChannel.addEventListener('open', () => {
      if (!file || !channel || sender) return
      phase.value = 'transferring'
      lastStatusAt = 0
      signaling?.sendStatus('transferring', receivedBytes.value)
      sender = new FileSender(dataChannel, file, {
        onAck(bytes) {
          receivedBytes.value = bytes
          if (room.value) room.value.bytesReceived = bytes
          persistProgress(bytes)
        },
        onDone() {
          window.clearTimeout(peerLeftTimer)
          const total = file?.size ?? 0
          receivedBytes.value = total
          if (room.value) {
            room.value.bytesReceived = total
            room.value.status = 'completed'
            room.value.completedAt = new Date().toISOString()
          }
          phase.value = 'completed'
          signaling?.sendStatus('completed', total)
          resetPeer()
        },
        onError(code) {
          errorCode.value = code
          phase.value = 'failed'
          signaling?.sendStatus('failed', receivedBytes.value)
        },
      })
      sender.start()
    })

    try {
      await connection.sendOffer()
    } catch {
      errorCode.value = 'connection_lost'
      phase.value = 'failed'
    }
  }

  async function applySignal(data: SignalData): Promise<void> {
    if (!peer) return
    try {
      if (data.sdp) {
        if (data.sdp.type === 'answer') await peer.applyAnswer(data.sdp)
        return
      }
      if (data.candidate) peer.addRemoteCandidate(data.candidate)
    } catch {
      errorCode.value = 'connection_lost'
      phase.value = 'failed'
    }
  }

  // Progress is persisted at most every two seconds to keep writes cheap.
  function persistProgress(bytes: number): void {
    const now = Date.now()
    if (now - lastStatusAt < 2000) return
    lastStatusAt = now
    signaling?.sendStatus('transferring', bytes)
  }

  function mergeStats(stats: RoomStats, events: RoomEvent[]): void {
    if (!room.value) return

    room.value = {
      ...room.value,
      status: stats.status,
      bytesReceived: Math.max(room.value.bytesReceived, stats.bytesReceived),
      failedAttempts: stats.failedAttempts,
      connectedAt: stats.connectedAt,
      completedAt: stats.completedAt,
      closedAt: stats.closedAt,
      events,
    }
    receivedBytes.value = Math.max(receivedBytes.value, stats.bytesReceived)

    if (stats.status === 'completed') {
      receivedBytes.value = room.value.fileSize
      room.value.bytesReceived = room.value.fileSize
      if (phase.value !== 'completed') {
        phase.value = 'completed'
        resetPeer()
      }
    }

    if (stats.status === 'closed') {
      phase.value = 'closed'
      resetPeer()
    }
  }

  function appendEvent(type: string): void {
    if (!room.value) return
    room.value.events = [...room.value.events, { type, createdAt: new Date().toISOString() }].slice(-50)
  }

  async function endSession(): Promise<void> {
    const closingToken = token
    const closingSecret = ownerSecret
    resetPeer()
    signaling?.close()
    signaling = null
    token = ''
    ownerSecret = ''
    try {
      if (closingToken && closingSecret) await closeRoom(closingToken, closingSecret)
    } catch {
      // the room may already be gone
    }
    reset()
  }

  function reset(): void {
    resetPeer()
    signaling?.close()
    signaling = null
    token = ''
    ownerSecret = ''
    room.value = null
    shareURL.value = ''
    peerOnline.value = false
    receivedBytes.value = 0
    errorCode.value = ''
    file = null
    phase.value = 'idle'
  }

  function resetPeer(): void {
    sender?.abort()
    sender = null
    if (channel) {
      channel.close()
      channel = null
    }
    peer?.close()
    peer = null
  }

  function mapError(error: unknown): string {
    if (error instanceof SignalingError) return error.code
    if (error instanceof ApiError) {
      if (error.code === 'invalid_input') {
        if (error.field === 'password') return 'password_too_short'
        if (error.field === 'fileSize') return 'file_too_large'
        return 'invalid_input'
      }
      return error.code
    }
    return 'generic'
  }

  onBeforeUnmount(() => {
    unmounted = true
    window.clearTimeout(peerLeftTimer)
    const closingToken = token
    const closingSecret = ownerSecret
    resetPeer()
    signaling?.close()
    signaling = null
    if (closingToken && closingSecret) closeRoomOnUnload(closingToken, closingSecret)
  })

  return {
    phase,
    room,
    shareURL,
    peerOnline,
    receivedBytes,
    errorCode,
    config,
    fileSize,
    progress,
    active,
    start,
    endSession,
    reset,
  }
}
