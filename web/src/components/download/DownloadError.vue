<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import AppButton from '@/components/AppButton.vue'

defineProps<{ message: string }>()

const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()
</script>

<template>
  <div class="space-y-4 text-center">
    <span class="mx-auto grid h-14 w-14 place-items-center rounded-full bg-red-50 text-red-700 ring-8 ring-red-50/50">
      <svg viewBox="0 0 24 24" class="h-7 w-7" fill="none" aria-hidden="true">
        <path d="M12 7v6m0 3.2v.3" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
        <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.8" />
      </svg>
    </span>
    <div class="space-y-1">
      <h2 class="text-lg font-extrabold tracking-tight text-ink">{{ t('download.errorTitle') }}</h2>
      <p class="mx-auto max-w-sm text-sm text-ink-soft">{{ message }}</p>
    </div>
    <div class="flex flex-col gap-2 sm:flex-row">
      <AppButton block @click="emit('retry')">{{ t('download.retry') }}</AppButton>
      <RouterLink to="/" class="btn-ghost justify-center">{{ t('common.backHome') }}</RouterLink>
    </div>
  </div>
</template>
