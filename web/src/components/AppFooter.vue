<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <footer class="bg-transparent">
    <div
      aria-hidden="true"
      class="h-px w-full bg-[radial-gradient(ellipse_at_center,var(--color-ink-soft)_0%,transparent_70%)] opacity-25"
    />
    <p
      class="mx-auto w-full max-w-5xl px-4 py-6 text-center text-xs text-ink-soft sm:px-6"
    >
      {{ t('footer.made') }}
    </p>
  </footer>
</template>
