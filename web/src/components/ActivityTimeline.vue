<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { RoomEvent } from '@/lib/api'
import { formatTime } from '@/lib/format'

const props = withDefaults(defineProps<{ events: RoomEvent[]; limit?: number }>(), {
  limit: 7,
})

const { t, te, locale } = useI18n()

const items = computed(() => props.events.slice(-props.limit).reverse())

// Each event type gets a dot colour, with a neutral default.
const DOT_COLORS: Record<string, string> = {
  transfer_completed: 'bg-emerald-500',
  wrong_password: 'bg-red-500',
  transfer_failed: 'bg-falafel-500',
  closed: 'bg-ink-soft/40',
  expired: 'bg-ink-soft/40',
}

function label(type: string): string {
  const key = `events.${type}`
  return te(key) ? t(key) : type
}

function dotClass(type: string): string {
  return DOT_COLORS[type] ?? 'bg-falafel-600'
}
</script>

<template>
  <TransitionGroup
    v-if="items.length"
    name="soft"
    tag="ul"
    class="relative space-y-3"
  >
    <li
      v-for="(event, index) in items"
      :key="`${event.type}-${event.createdAt}-${index}`"
      class="flex items-start gap-3"
    >
      <span class="relative mt-1.5 flex h-2 w-2 shrink-0">
        <span class="h-2 w-2 rounded-full" :class="dotClass(event.type)" />
      </span>
      <span class="flex min-w-0 flex-1 items-baseline justify-between gap-3">
        <span class="text-xs font-medium text-ink">{{ label(event.type) }}</span>
        <time class="shrink-0 text-[0.65rem] tabular-nums text-ink-soft" :datetime="event.createdAt">
          {{ formatTime(event.createdAt, locale) }}
        </time>
      </span>
    </li>
  </TransitionGroup>
  <p v-else class="text-xs text-ink-soft">{{ t('stats.empty') }}</p>
</template>
