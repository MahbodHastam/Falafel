type Listener = (event: Event) => void

// A pair of in-memory data channels wired back to back, for testing both ends of a transfer.
export class FakeChannel {
  readyState: RTCDataChannelState = 'open'
  bufferedAmount = 0
  bufferedAmountLowThreshold = 0
  binaryType: BinaryType = 'arraybuffer'
  sent: Array<string | ArrayBuffer> = []
  peer: FakeChannel | null = null
  // When false, sent frames are recorded but never delivered.
  deliver = true

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
    this.sent.push(typeof data === 'string' ? data : data.slice(0))
    if (this.deliver) this.peer?.receive(data)
  }

  receive(data: string | ArrayBuffer): void {
    queueMicrotask(() => this.emit('message', new MessageEvent('message', { data })))
  }

  close(): void {
    this.readyState = 'closed'
    this.emit('close', new Event('close'))
  }

  emit(type: string, event: Event): void {
    for (const listener of [...(this.listeners.get(type) ?? [])]) listener(event)
  }

  strings(): Array<Record<string, unknown>> {
    return this.sent.filter((m): m is string => typeof m === 'string').map((m) => JSON.parse(m))
  }
}

export function channelPair(): [FakeChannel, FakeChannel] {
  const a = new FakeChannel()
  const b = new FakeChannel()
  a.peer = b
  b.peer = a
  return [a, b]
}
