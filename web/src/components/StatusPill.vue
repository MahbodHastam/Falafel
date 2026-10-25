<script setup lang="ts">
withDefaults(
  defineProps<{
    tone?: 'waiting' | 'active' | 'success' | 'error' | 'neutral'
  }>(),
  { tone: 'neutral' },
)

const tones: Record<string, string> = {
  waiting: 'border-falafel-700/20 bg-falafel-100/80 text-falafel-900',
  active: 'border-falafel-600/25 bg-falafel-100/80 text-falafel-900',
  success: 'border-emerald-700/20 bg-emerald-50 text-emerald-800',
  error: 'border-red-700/20 bg-red-50 text-red-800',
  neutral: 'border-falafel-900/10 bg-white/70 text-ink-soft',
}

const dots: Record<string, string> = {
  waiting: 'bg-falafel-600 animate-pulse-soft',
  active: 'bg-falafel-600 animate-pulse-soft',
  success: 'bg-emerald-600',
  error: 'bg-red-600',
  neutral: 'bg-ink-soft/60',
}
</script>

<template>
  <span
    class="inline-flex items-center gap-2 rounded-full border px-3 py-1 text-xs font-semibold"
    :class="tones[tone]"
  >
    <span class="h-1.5 w-1.5 rounded-full" :class="dots[tone]" />
    <slot />
  </span>
</template>
