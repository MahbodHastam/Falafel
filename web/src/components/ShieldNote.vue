<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <p class="flex items-center justify-center gap-1.5 text-center text-[0.68rem] text-ink-soft">
    <svg viewBox="0 0 20 20" class="h-3 w-3" fill="none" aria-hidden="true">
      <path
        d="M10 2.5 16 5v5c0 3.6-2.6 6.3-6 7.5-3.4-1.2-6-3.9-6-7.5V5l6-2.5Z"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linejoin="round"
      />
    </svg>
    {{ t('common.noUpload') }}
  </p>
</template>
