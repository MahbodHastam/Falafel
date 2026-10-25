<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  title: string
  subtitle: string
  hint: string
}>()

const emit = defineEmits<{ select: [file: File] }>()

const dragging = ref(false)
const input = ref<HTMLInputElement | null>(null)

function pick(): void {
  input.value?.click()
}

function accept(list: FileList | null): void {
  const file = list?.[0]
  if (file) emit('select', file)
}

function onDrop(event: DragEvent): void {
  dragging.value = false
  accept(event.dataTransfer?.files ?? null)
}
</script>

<template>
  <div
    class="group relative rounded-3xl border-2 border-dashed border-falafel-800/20 bg-falafel-50/40 p-1 transition duration-300"
    :class="dragging ? 'border-falafel-500 bg-falafel-100/80 shadow-[0_0_0_6px_rgba(242,159,5,0.12)]' : 'hover:border-falafel-600/40 hover:bg-falafel-50/80'"
    @dragenter.prevent="dragging = true"
    @dragover.prevent="dragging = true"
    @dragleave.prevent="dragging = false"
    @drop.prevent="onDrop"
  >
    <button
      type="button"
      class="flex w-full cursor-pointer flex-col items-center gap-3 rounded-[1.35rem] px-6 py-10 text-center focus-visible:outline-none"
      @click="pick"
    >
      <span
        class="grid h-14 w-14 place-items-center rounded-2xl bg-white/80 text-falafel-700 ring-1 ring-falafel-900/5 transition duration-300"
      >
        <svg viewBox="0 0 24 24" class="h-6 w-6" fill="none" aria-hidden="true">
          <path d="M12 16V4m0 0L7.5 8.5M12 4l4.5 4.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
          <path d="M4 15v3.5A1.5 1.5 0 0 0 5.5 20h13a1.5 1.5 0 0 0 1.5-1.5V15" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
        </svg>
      </span>
      <span class="space-y-1">
        <span class="block text-base font-bold text-ink">{{ title }}</span>
        <span class="block text-sm text-ink-soft">{{ subtitle }}</span>
      </span>
      <span class="rounded-full bg-white/70 px-3 py-1 text-[0.7rem] font-medium text-ink-soft ring-1 ring-falafel-900/5">
        {{ hint }}
      </span>
    </button>
    <input
      ref="input"
      type="file"
      class="sr-only"
      tabindex="-1"
      aria-hidden="true"
      @change="accept(($event.target as HTMLInputElement).files)"
    />
  </div>
</template>
