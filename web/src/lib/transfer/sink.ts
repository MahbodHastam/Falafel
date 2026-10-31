export interface DownloadSink {
  write(chunk: ArrayBuffer): Promise<void>
  close(): Promise<void>
  abort(): Promise<void>
}

// Large files go through the file picker so they never sit in memory as one blob.
const FS_THRESHOLD = 256 * 1024 * 1024

const MAX_NAME_LENGTH = 200
const WINDOWS_RESERVED = /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(\.|$)/i

// Makes a sender-supplied name safe to hand to the OS: no path separators, control
// or direction-override characters (which can disguise "fdp.exe" as "exe.pdf"),
// no Windows reserved names or trailing dots, and a bounded length.
export function sanitizeFileName(name: string): string {
  let cleaned = name
    .replace(/[\u0000-\u001f\u007f\u202a-\u202e\u2066-\u2069]/g, '')
    .replace(/[\\/:*?"<>|]/g, '_')
    .trim()
    .replace(/[. ]+$/, '')
  if (WINDOWS_RESERVED.test(cleaned)) cleaned = `_${cleaned}`
  if (cleaned.length > MAX_NAME_LENGTH) {
    const dot = cleaned.lastIndexOf('.')
    const ext = dot > 0 && cleaned.length - dot <= 16 ? cleaned.slice(dot) : ''
    cleaned = cleaned.slice(0, MAX_NAME_LENGTH - ext.length) + ext
  }
  return cleaned || 'falafel-file'
}

// Returns null when the user cancels the save dialog.
export async function createDownloadSink(
  fileName: string,
  fileSize: number,
  mimeType: string,
): Promise<DownloadSink | null> {
  const safeName = sanitizeFileName(fileName)
  const picker = window.showSaveFilePicker

  if (picker && fileSize >= FS_THRESHOLD) {
    try {
      const handle = await picker({ suggestedName: safeName })
      return new FileSystemSink(handle)
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') return null
      // Any other picker failure falls back to the blob sink below.
    }
  }

  return new BlobSink(safeName, mimeType)
}

// Chunks are folded into a Blob every FOLD_BYTES. Browsers keep large Blobs in
// disk-backed storage, so this keeps the JavaScript heap small on browsers that
// lack the File System Access API.
const FOLD_BYTES = 64 * 1024 * 1024

class BlobSink implements DownloadSink {
  private readonly name: string
  private readonly type: string
  private parts: BlobPart[] = []
  private pending = 0
  private finished = false

  constructor(name: string, type: string) {
    this.name = name
    this.type = type || 'application/octet-stream'
  }

  async write(chunk: ArrayBuffer): Promise<void> {
    if (this.finished) throw new Error('sink_closed')
    this.parts.push(chunk)
    this.pending += chunk.byteLength
    if (this.pending >= FOLD_BYTES) {
      this.parts = [new Blob(this.parts)]
      this.pending = 0
    }
  }

  async close(): Promise<void> {
    if (this.finished) return
    this.finished = true
    const blob = new Blob(this.parts, { type: this.type })
    this.parts = []
    triggerDownload(blob, this.name)
  }

  async abort(): Promise<void> {
    this.finished = true
    this.parts = []
  }
}

class FileSystemSink implements DownloadSink {
  private readonly handle: FalafelFileHandle
  private writable: FalafelWritable | null = null
  private finished = false

  constructor(handle: FalafelFileHandle) {
    this.handle = handle
  }

  async write(chunk: ArrayBuffer): Promise<void> {
    if (this.finished) throw new Error('sink_closed')
    if (!this.writable) {
      this.writable = await this.handle.createWritable()
    }
    await this.writable.write(chunk)
  }

  async close(): Promise<void> {
    if (this.finished) return
    this.finished = true
    await this.writable?.close()
  }

  async abort(): Promise<void> {
    if (this.finished) return
    this.finished = true
    try {
      await this.writable?.abort()
    } catch {
      // The writable may already be closed, and there is nothing left to clean up.
    }
  }
}

function triggerDownload(blob: Blob, fileName: string): void {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = fileName
  anchor.rel = 'noopener'
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)
  // Keep the blob URL alive for a minute; revoking it immediately can cancel the download.
  window.setTimeout(() => URL.revokeObjectURL(url), 60_000)
}
