<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { OwnerRoom } from '@/lib/api'
import type { SendPhase } from '@/composables/useSendSession'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { formatBytes } from '@/lib/format'
import ActivityTimeline from './ActivityTimeline.vue'
import ProgressBar from './ProgressBar.vue'
import StatusPill from './StatusPill.vue'

const props = defineProps<{
  room: OwnerRoom
  phase: SendPhase
  errorCode: string
  receivedBytes: number
}>()

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const total = computed(() => props.room.fileSize)
const received = computed(() => Math.max(props.receivedBytes, props.room.bytesReceived))
const fraction = computed(() => (total.value > 0 ? received.value / total.value : 0))

const pending = computed(() =>
  props.room.status === 'completed' || props.room.status === 'closed' ? 0 : 1,
)
const completed = computed(() => (props.room.status === 'completed' ? 1 : 0))
const wrongPasswords = computed(() => props.room.failedAttempts)

const tone = computed<'waiting' | 'active' | 'success' | 'error' | 'neutral'>(() => {
  switch (props.phase) {
    case 'completed':
      return 'success'
    case 'failed':
      return 'error'
    case 'transferring':
    case 'connecting':
      return 'active'
    case 'closed':
      return 'neutral'
    default:
      return 'waiting'
  }
})

const statusText = computed(() => {
  switch (props.phase) {
    case 'waiting':
      return t('send.status.waiting')
    case 'connecting':
      return t('send.status.connecting')
    case 'transferring':
      return t('send.status.transferring')
    case 'completed':
      return t('send.status.completed')
    case 'closed':
      return t('send.status.closed')
    case 'failed':
      return props.errorCode === 'peer_left'
        ? t('send.status.receiverLeft')
        : errorMessage(props.errorCode)
    default:
      return t('send.status.waiting')
  }
})

const showProgress = computed(
  () =>
    props.phase === 'transferring' ||
    props.phase === 'connecting' ||
    (received.value > 0 && props.phase !== 'waiting'),
)

const percent = computed(() => `${Math.round(fraction.value * 100)}%`)

const cards = computed(() => [
  { label: t('stats.pending'), value: pending.value, tone: 'text-falafel-800' },
  { label: t('stats.completed'), value: completed.value, tone: 'text-emerald-700' },
  { label: t('stats.wrongPassword'), value: wrongPasswords.value, tone: 'text-red-700' },
])
</script>

<template>
  <section class="surface space-y-5 p-5 sm:p-6">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <StatusPill :tone="tone">{{ statusText }}</StatusPill>
    </div>

    <dl class="grid grid-cols-3 gap-2.5">
      <div
        v-for="card in cards"
        :key="card.label"
        class="rounded-2xl border border-falafel-900/10 bg-white/55 px-3 py-3"
      >
        <dt class="min-h-[1.9rem] text-[0.62rem] font-semibold uppercase leading-tight tracking-widest text-ink-soft">
          {{ card.label }}
        </dt>
        <dd class="mt-1 text-2xl font-extrabold tabular-nums" :class="card.tone">
          {{ card.value }}
        </dd>
      </div>
    </dl>

    <div v-if="showProgress" class="space-y-2">
      <div class="flex items-baseline justify-between gap-2">
        <span class="text-xs font-semibold text-ink">
          {{ phase === 'completed' ? t('send.progress.sent') : t('send.status.transferring') }}
        </span>
        <span class="text-xs font-bold tabular-nums text-falafel-800">{{ percent }}</span>
      </div>
      <ProgressBar :value="fraction" :active="phase === 'transferring'" />
      <p class="text-[0.68rem] tabular-nums text-ink-soft">
        {{
          t('send.progress.of', {
            done: formatBytes(received, locale),
            total: formatBytes(total, locale),
          })
        }}
      </p>
    </div>

    <div class="hairline" />

    <div>
      <h3 class="card-label mb-3">{{ t('stats.activity') }}</h3>
      <ActivityTimeline :events="room.events" />
    </div>
  </section>
</template>
