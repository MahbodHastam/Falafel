<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import AppButton from '@/components/AppButton.vue'

defineProps<{
  savedName: string
  size: string
  duration: string
  averageSpeed: string
}>()

const emit = defineEmits<{ done: [] }>()
const { t } = useI18n()
</script>

<template>
  <div class="space-y-5">
    <div class="flex flex-col items-center gap-3 text-center">
      <span
        class="animate-success grid h-16 w-16 place-items-center rounded-full bg-emerald-100 text-emerald-700 ring-8 ring-emerald-50"
      >
        <svg viewBox="0 0 24 24" class="h-8 w-8" fill="none" aria-hidden="true">
          <path d="m5 12.5 4.5 4.5L19 7" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </span>
      <div>
        <h2 class="text-xl font-extrabold tracking-tight text-ink">{{ t('download.completed') }}</h2>
        <p class="mt-1 text-xs text-ink-soft">
          {{ t('download.saved') }}: <span class="font-semibold text-ink">{{ savedName }}</span>
        </p>
      </div>
    </div>

    <dl class="grid grid-cols-3 gap-2 text-center">
      <div class="rounded-2xl bg-falafel-50/80 px-2 py-3 ring-1 ring-falafel-900/5">
        <dt class="text-[0.6rem] font-semibold uppercase tracking-wider text-ink-soft">
          {{ t('download.size') }}
        </dt>
        <dd class="mt-1 text-sm font-bold tabular-nums text-ink">{{ size }}</dd>
      </div>
      <div class="rounded-2xl bg-falafel-50/80 px-2 py-3 ring-1 ring-falafel-900/5">
        <dt class="text-[0.6rem] font-semibold uppercase tracking-wider text-ink-soft">
          {{ t('download.time') }}
        </dt>
        <dd class="mt-1 text-sm font-bold tabular-nums text-ink">{{ duration }}</dd>
      </div>
      <div class="rounded-2xl bg-falafel-50/80 px-2 py-3 ring-1 ring-falafel-900/5">
        <dt class="text-[0.6rem] font-semibold uppercase tracking-wider text-ink-soft">
          {{ t('download.speed') }}
        </dt>
        <dd class="mt-1 text-sm font-bold tabular-nums text-ink">{{ averageSpeed }}</dd>
      </div>
    </dl>

    <AppButton block @click="emit('done')">{{ t('download.done') }}</AppButton>
  </div>
</template>
