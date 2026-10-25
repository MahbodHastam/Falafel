<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import ProgressBar from '@/components/ProgressBar.vue'
import StatusPill from '@/components/StatusPill.vue'

defineProps<{
  status: string
  speed: string
  percent: string
  received: string
  total: string
  value: number
  active: boolean
  eta: string
}>()

const emit = defineEmits<{ cancel: [] }>()
const { t } = useI18n()
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-2">
      <StatusPill tone="active">{{ status }}</StatusPill>
      <span v-if="speed" class="text-xs font-semibold tabular-nums text-falafel-800">
        {{ speed }}
      </span>
    </div>

    <div class="flex items-end justify-between gap-3">
      <p class="text-4xl font-extrabold tabular-nums tracking-tight text-ink">{{ percent }}</p>
      <p class="pb-1 text-[0.68rem] tabular-nums text-ink-soft">
        {{ t('download.receivedOf', { done: received, total }) }}
      </p>
    </div>

    <ProgressBar :value="value" :active="active" />

    <div class="flex items-center justify-between gap-2 text-[0.68rem] text-ink-soft">
      <span>{{ eta || t('download.keepOpen') }}</span>
      <button
        type="button"
        class="cursor-pointer font-semibold text-falafel-800 underline-offset-4 hover:underline"
        @click="emit('cancel')"
      >
        {{ t('download.cancel') }}
      </button>
    </div>
  </div>
</template>
