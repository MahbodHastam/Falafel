/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>
  export default component
}

interface FalafelWritable {
  write(data: BufferSource | Blob | string): Promise<void>
  close(): Promise<void>
  abort(): Promise<void>
}

interface FalafelFileHandle {
  createWritable(): Promise<FalafelWritable>
}

interface Window {
  showSaveFilePicker?: (options?: {
    suggestedName?: string
    types?: Array<{ description?: string; accept: Record<string, string[]> }>
  }) => Promise<FalafelFileHandle>
}
