import type { SignalData } from './signaling'
import { dtlsFingerprint } from './transfer/channel-auth'

/**
 * A single WebRTC connection plus the ICE bookkeeping both sides need.
 *
 * The sender builds the offer and the receiver answers it. Network candidates
 * arrive asynchronously and may show up before the remote description exists,
 * so they wait in `pendingCandidates` until there is somewhere to put them.
 */

interface PeerLinkOptions {
  iceServers: RTCIceServer[]
  sendSignal(data: SignalData): void
  onFailed(): void
}

export class PeerLink {
  readonly connection: RTCPeerConnection

  private readonly options: PeerLinkOptions
  private pendingCandidates: RTCIceCandidateInit[] = []

  constructor(options: PeerLinkOptions) {
    this.options = options
    this.connection = new RTCPeerConnection({ iceServers: options.iceServers })

    this.connection.addEventListener('icecandidate', (event) => {
      if (event.candidate) {
        options.sendSignal({ candidate: event.candidate.toJSON() })
      }
    })

    this.connection.addEventListener('connectionstatechange', () => {
      if (this.connection.connectionState === 'failed') {
        options.onFailed()
      }
    })
  }

  /** Sender side: create an offer and send it to the receiver. */
  async sendOffer(): Promise<void> {
    const offer = await this.connection.createOffer()
    await this.connection.setLocalDescription(offer)
    this.sendLocalDescription()
  }

  /**
   * Receiver side: accept the offer, reply with an answer, then apply the
   * candidates collected while we were still setting up.
   */
  async acceptOffer(offer: RTCSessionDescriptionInit): Promise<void> {
    await this.connection.setRemoteDescription(offer)
    const answer = await this.connection.createAnswer()
    await this.connection.setLocalDescription(answer)
    this.sendLocalDescription()
    await this.flushCandidates()
  }

  /** Sender side: apply the receiver's answer to the offer we sent. */
  async applyAnswer(answer: RTCSessionDescriptionInit): Promise<void> {
    if (this.connection.signalingState !== 'have-local-offer') return
    await this.connection.setRemoteDescription(answer)
    await this.flushCandidates()
  }

  addRemoteCandidate(candidate: RTCIceCandidateInit): void {
    if (this.connection.remoteDescription) {
      void this.connection.addIceCandidate(candidate).catch(() => undefined)
      return
    }
    this.pendingCandidates.push(candidate)
  }

  /** DTLS fingerprints of both ends, or null until both descriptions are set. */
  fingerprints(): { local: string; remote: string } | null {
    const local = dtlsFingerprint(this.connection.localDescription?.sdp ?? '')
    const remote = dtlsFingerprint(this.connection.remoteDescription?.sdp ?? '')
    return local && remote ? { local, remote } : null
  }

  close(): void {
    this.pendingCandidates = []
    this.connection.close()
  }

  private async flushCandidates(): Promise<void> {
    const queued = this.pendingCandidates
    this.pendingCandidates = []
    for (const candidate of queued) {
      await this.connection.addIceCandidate(candidate).catch(() => undefined)
    }
  }

  private sendLocalDescription(): void {
    const description = this.connection.localDescription
    if (!description) return
    this.options.sendSignal({ sdp: { type: description.type, sdp: description.sdp } })
  }
}
