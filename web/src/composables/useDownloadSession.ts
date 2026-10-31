import { computed, onBeforeUnmount, ref } from 'vue'

import {
  ApiError,
  getConfig,
  getRoom,
  joinRoom,
  type AppConfig,
  type PublicRoom,
} from '@/lib/api'
import { PeerLink } from '@/lib/peer'
import { SignalingClient, SignalingError, type SignalingEvent } from '@/lib/signaling'
import { createDownloadSink, sanitizeFileName, type DownloadSink } from '@/lib/transfer/sink'
import { authenticateChannel } from '@/lib/transfer/channel-auth'
import { FileReceiver } from '@/lib/transfer/receiver'

export type DownloadPhase =
  | 'loading'
  | 'ready'
  | 'password'
  | 'starting'
  | 'connecting'
  | 'transferring'
  | 'completed'
  | 'error'
  | 'not_found'

// Keep the socket open briefly after completing so the sender receives the
// final status message before the connection goes away.
const TEARDOWN_DELAY_MS = 800

/**
 * Drives the receiving side: load the room, ask for a password when needed,
 * then accept the sender's offer and verify the incoming file.
 */
export function useDownloadSession(token: string, linkKey: string | null) {
  const phase = ref<DownloadPhase>('loading')
  const room = ref<PublicRoom | null>(null)
  const errorCode = ref('')
  const retryAfter = ref(0)
  const bytes = ref(0)
  const progress = ref(0)
  const speed = ref(0)
  const etaMs = ref(0)
  const elapsedMs = ref(0)
  const averageSpeed = ref(0)
  const sha256 = ref('')
  const savedName = ref('')

  let config: AppConfig | null = null
  let signaling: SignalingClient | null = null
  let peer: PeerLink | null = null
  let channel: RTCDataChannel | null = null
  let receiver: FileReceiver | null = null
  let sink: DownloadSink | null = null
  // Candidates can reach us before the offer has been processed, so keep them
  // until the peer connection exists.
  let earlyCandidates: RTCIceCandidateInit[] = []
  let startedAt = 0
  let sampleAt = 0
  let sampleBytes = 0
  let unmounted = false
  let loading = false
  let teardownTimer: number | undefined

  const size = computed(() => room.value?.fileSize ?? 0)
  const displayName = computed(() => sanitizeFileName(room.value?.fileName ?? ''))
  const active = computed(
    () => phase.value === 'connecting' || phase.value === 'transferring',
  )

  async function load(): Promise<void> {
    if (loading) return
    loading = true
    phase.value = 'loading'
    try {
      room.value = await getRoom(token)
      phase.value = room.value.passwordRequired ? 'password' : 'ready'
    } catch (error) {
      if (error instanceof ApiError && error.status === 404) {
        phase.value = 'not_found'
      } else {
        errorCode.value = mapError(error)
        phase.value = 'error'
      }
    } finally {
      loading = false
    }
  }

  async function start(password: string): Promise<void> {
    if (!room.value) return
    if (active.value || phase.value === 'starting') return

    if (typeof RTCPeerConnection === 'undefined') {
      errorCode.value = 'unsupported'
      phase.value = 'error'
      return
    }

    resetProgress()
    if (!linkKey) {
      // Without the key from the link's #fragment the sender cannot be verified.
      errorCode.value = 'link_incomplete'
      phase.value = 'error'
      return
    }
    phase.value = 'starting'

    try {
      config = await getConfig()

      sink = await createDownloadSink(room.value.fileName, room.value.fileSize, room.value.mimeType)
      if (!sink) {
        // The user closed the save dialog.
        phase.value = room.value.passwordRequired ? 'password' : 'ready'
        return
      }

      const joined = await joinRoom(token, password)
      if (unmounted) return

      signaling = new SignalingClient(token, { role: 'downloader', ticket: joined.ticket })
      signaling.onEvent(handleEvent)
      await signaling.connect()
      if (unmounted) return
      phase.value = 'connecting'
    } catch (error) {
      if (unmounted) return
      releaseTransfer()

      if (error instanceof ApiError && error.code === 'wrong_password') {
        errorCode.value = 'wrong_password'
        phase.value = room.value.passwordRequired ? 'password' : 'ready'
        return
      }
      if (error instanceof ApiError && error.code === 'too_many_attempts') {
        retryAfter.value = error.retryAfter ?? 60
        errorCode.value = 'too_many_attempts'
        phase.value = room.value.passwordRequired ? 'password' : 'ready'
        return
      }

      errorCode.value = mapError(error)
      phase.value = errorCode.value === 'not_found' ? 'not_found' : 'error'
    }
  }

  function handleEvent(event: SignalingEvent): void {
    if (unmounted) return
    switch (event.type) {
      case 'ready':
        phase.value = 'connecting'
        break
      case 'signal':
        if (event.data.sdp?.type === 'offer') {
          void acceptOffer(event.data.sdp)
        } else if (event.data.candidate) {
          if (peer) peer.addRemoteCandidate(event.data.candidate)
          else earlyCandidates.push(event.data.candidate)
        }
        break
      case 'closed':
        if (phase.value !== 'completed') fail('owner_left')
        break
      case 'error':
        fail(event.code)
        break
    }
  }

  async function acceptOffer(offer: RTCSessionDescriptionInit): Promise<void> {
    if (!config || !sink || !room.value || peer) return

    const connection = new PeerLink({
      iceServers: config.iceServers,
      sendSignal: (data) => signaling?.sendSignal(data),
      onFailed: () => {
        if (phase.value !== 'completed') fail('ice_failed')
      },
    })
    peer = connection
    phase.value = 'connecting'

    for (const candidate of earlyCandidates) connection.addRemoteCandidate(candidate)
    earlyCandidates = []

    connection.connection.addEventListener('datachannel', (event) => {
      if (!sink || !room.value || !linkKey) return
      const dataChannel = event.channel
      channel = dataChannel
      const currentSink = sink
      const total = room.value.fileSize
      const fingerprints = connection.fingerprints()
      if (!fingerprints) {
        fail('verification_failed')
        return
      }

      // The handshake starts synchronously so the sender's first message is not missed.
      authenticateChannel({
        channel: dataChannel,
        key: linkKey,
        role: 'receiver',
        senderFingerprint: fingerprints.remote,
        receiverFingerprint: fingerprints.local,
      }).then(
        () => {
          if (channel !== dataChannel || unmounted) return
          receiver = new FileReceiver(dataChannel, currentSink, total, {
            onProgress(received) {
              updateProgress(received, total)
            },
            onComplete(result) {
              finish(result.sha256, result.size)
            },
            onError(code) {
              fail(code)
            },
          })
          receiver.start()
        },
        () => {
          if (channel === dataChannel) fail('verification_failed')
        },
      )
    })

    try {
      await connection.acceptOffer(offer)
    } catch {
      fail('connection_lost')
    }
  }

  function updateProgress(received: number, total: number): void {
    if (phase.value !== 'transferring') {
      phase.value = 'transferring'
      startedAt = Date.now()
      sampleAt = startedAt
      sampleBytes = 0
    }

    bytes.value = received
    progress.value = total > 0 ? Math.min(1, received / total) : 0
    elapsedMs.value = Date.now() - startedAt

    const now = Date.now()
    const elapsed = (now - sampleAt) / 1000
    if (elapsed < 0.4) return

    // Blend the latest sample with the running average so the number does not
    // jump around on a noisy connection.
    const instant = (received - sampleBytes) / elapsed
    speed.value = speed.value > 0 ? speed.value * 0.65 + instant * 0.35 : instant
    sampleAt = now
    sampleBytes = received

    if (speed.value > 1) {
      etaMs.value = Math.max(0, ((total - received) / speed.value) * 1000)
    }
  }

  function finish(hash: string, received: number): void {
    if (unmounted) return

    const duration = startedAt > 0 ? Date.now() - startedAt : 0
    phase.value = 'completed'
    bytes.value = received
    progress.value = 1
    elapsedMs.value = duration
    averageSpeed.value = duration > 0 ? received / (duration / 1000) : 0
    sha256.value = hash
    savedName.value = displayName.value

    signaling?.sendStatus('completed', received)
    window.clearTimeout(teardownTimer)
    teardownTimer = window.setTimeout(() => {
      if (phase.value !== 'completed') return
      signaling?.close()
      signaling = null
      closePeer()
    }, TEARDOWN_DELAY_MS)
  }

  function fail(code: string): void {
    if (unmounted || phase.value === 'completed') return
    window.clearTimeout(teardownTimer)
    errorCode.value = code
    phase.value = code === 'not_found' ? 'not_found' : 'error'
    releaseTransfer()
  }

  function cancel(): void {
    receiver?.abort()
    receiver = null
    releaseTransfer()
    errorCode.value = 'cancelled'
    phase.value = 'error'
  }

  function retry(): void {
    releaseTransfer()
    resetProgress()
    retryAfter.value = 0
    phase.value = room.value?.passwordRequired ? 'password' : 'ready'
  }

  function resetProgress(): void {
    errorCode.value = ''
    retryAfter.value = 0
    bytes.value = 0
    progress.value = 0
    speed.value = 0
    etaMs.value = 0
    elapsedMs.value = 0
    averageSpeed.value = 0
    sha256.value = ''
  }

  // Drop the socket, peer connection and partial file in one place so every
  // exit path leaves the same clean state.
  function releaseTransfer(): void {
    signaling?.close()
    signaling = null
    receiver = null
    void sink?.abort()
    sink = null
    closePeer()
  }

  function closePeer(): void {
    earlyCandidates = []
    if (channel) {
      channel.close()
      channel = null
    }
    peer?.close()
    peer = null
  }

  function mapError(error: unknown): string {
    if (error instanceof ApiError) return error.code
    if (error instanceof SignalingError) return error.code
    return 'generic'
  }

  onBeforeUnmount(() => {
    unmounted = true
    window.clearTimeout(teardownTimer)
    receiver?.abort()
    releaseTransfer()
  })

  return {
    phase,
    room,
    errorCode,
    retryAfter,
    bytes,
    progress,
    speed,
    etaMs,
    elapsedMs,
    averageSpeed,
    sha256,
    savedName,
    size,
    displayName,
    active,
    load,
    start,
    cancel,
    retry,
  }
}
