import { describe, expect, it } from 'vitest'

import { encodeControl, parseControl } from '@/lib/transfer/protocol'

describe('control protocol', () => {
  it('round-trips metadata', () => {
    const message = {
      type: 'meta' as const,
      name: 'falafel.pdf',
      size: 1234,
      mime: 'application/pdf',
      chunkSize: 65536,
    }
    expect(parseControl(encodeControl(message))).toEqual(message)
  })

  it('parses acknowledgements, hashes and completions', () => {
    expect(parseControl('{"type":"ack","bytes":42}')).toEqual({ type: 'ack', bytes: 42 })
    expect(parseControl('{"type":"end","sha256":"abc"}')).toEqual({ type: 'end', sha256: 'abc' })
    expect(parseControl('{"type":"done","ok":false,"error":"hash_mismatch"}')).toEqual({
      type: 'done',
      ok: false,
      error: 'hash_mismatch',
    })
  })

  it('rejects malformed payloads', () => {
    expect(parseControl('not json')).toBeNull()
    expect(parseControl('null')).toBeNull()
    expect(parseControl('{"type":"meta","name":"x"}')).toBeNull()
    expect(parseControl('{"type":"ack","bytes":"nope"}')).toBeNull()
    expect(parseControl('{"type":"unknown"}')).toBeNull()
  })
})
