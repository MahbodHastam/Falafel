export interface DownloadSink {
  write(chunk: ArrayBuffer): Promise<void>
  close(): Promise<void>
  abort(): Promise<void>
}

// Large files go through the file picker so they never sit in memory as one blob.
const FS_THRESHOLD = 256 * 1024 * 1024

export function sanitizeFileName(name: string): string {
  const cleaned = name
    .replace(/[\u0000-\u001f\u007f]/g, '')
    .replace(/[\\/:*?"<>|]/g, '_')
    .trim()
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

class BlobSink implements DownloadSink {
  private readonly name: string
  private readonly type: string
  private chunks: ArrayBuffer[] = []
  private finished = false

  constructor(name: string, type: string) {
    this.name = name
    this.type = type || 'application/octet-stream'
  }

  async write(chunk: ArrayBuffer): Promise<void> {
    if (this.finished) throw new Error('sink_closed')
    this.chunks.push(chunk)
  }

  async close(): Promise<void> {
    if (this.finished) return
    this.finished = true
    const blob = new Blob(this.chunks, { type: this.type })
    this.chunks = []
    triggerDownload(blob, this.name)
  }

  async abort(): Promise<void> {
    this.finished = true
    this.chunks = []
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
