<script setup lang="ts">
import { useToast } from '@/composables/useToast'

const { toasts, dismiss } = useToast()

const toneClass: Record<string, string> = {
  success: 'border-emerald-700/20 bg-emerald-50 text-emerald-900',
  error: 'border-red-700/20 bg-red-50 text-red-900',
  info: 'border-falafel-700/20 bg-falafel-50 text-falafel-900',
}
</script>

<template>
  <div
    class="pointer-events-none fixed inset-x-0 bottom-5 z-50 flex justify-center px-4"
    aria-live="polite"
    aria-atomic="true"
  >
    <TransitionGroup name="toast" tag="div" class="flex flex-col items-center gap-2">
      <button
        v-for="toast in toasts"
        :key="toast.id"
        type="button"
        class="pointer-events-auto inline-flex cursor-pointer items-center gap-2 rounded-full border px-4 py-2 text-xs font-semibold shadow-lg shadow-falafel-900/10 backdrop-blur-md"
        :class="toneClass[toast.tone]"
        @click="dismiss(toast.id)"
      >
        <svg
          v-if="toast.tone === 'success'"
          viewBox="0 0 20 20"
          class="h-3.5 w-3.5"
          fill="none"
          aria-hidden="true"
        >
          <path d="m4 10.5 4 4 8-9" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <span>{{ toast.message }}</span>
      </button>
    </TransitionGroup>
  </div>
</template>
