<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LogoMark from './LogoMark.vue'
import LanguageToggle from './LanguageToggle.vue'

const { t } = useI18n()

const repoURL = 'https://github.com/MahbodHastam/falafel'
</script>

<template>
  <header
    class="sticky top-4 z-30 mx-auto mt-4 flex w-fit max-w-[calc(100vw-2rem)] items-center justify-center gap-3 rounded-full border border-falafel-900/10 bg-cream/80 px-4 py-2 shadow-sm backdrop-blur-xl sm:gap-4 sm:px-6 sm:py-2.5 dark:border-white/10 dark:bg-falafel-950/80"
  >
    <RouterLink to="/" class="group flex cursor-pointer items-center gap-2.5 rounded-full">
      <LogoMark
        class="h-9 w-9 transition-transform duration-300 group-hover:-rotate-6 group-hover:scale-105"
      />
      <span class="flex flex-col leading-tight">
        <span class="text-base font-extrabold tracking-tight">{{ t('app.name') }}</span>
        <span v-if="t('app.tagline')" class="hidden text-[0.65rem] font-medium text-ink-soft sm:block">
          {{ t('app.tagline') }}
        </span>
      </span>
    </RouterLink>

    <span class="h-5 w-px bg-falafel-900/10" aria-hidden="true" />
    <div class="flex items-center gap-1.5">
      <LanguageToggle />
      <a
        :href="repoURL"
        target="_blank"
        rel="noreferrer noopener"
        class="btn-quiet px-2.5"
        :aria-label="t('nav.github')"
        :title="t('nav.github')"
      >
        <svg viewBox="0 0 24 24" class="h-5 w-5" fill="currentColor" aria-hidden="true">
          <path
            d="M12 2C6.48 2 2 6.58 2 12.25c0 4.53 2.87 8.37 6.84 9.73.5.1.68-.22.68-.49 0-.24-.01-1.04-.01-1.89-2.78.62-3.37-1.21-3.37-1.21-.45-1.18-1.11-1.49-1.11-1.49-.91-.63.07-.62.07-.62 1 .07 1.53 1.06 1.53 1.06.89 1.56 2.34 1.11 2.91.85.09-.66.35-1.11.63-1.36-2.22-.26-4.56-1.14-4.56-5.06 0-1.12.39-2.03 1.03-2.75-.1-.26-.45-1.3.1-2.71 0 0 .84-.28 2.75 1.05a9.36 9.36 0 0 1 5 0c1.91-1.33 2.75-1.05 2.75-1.05.55 1.41.2 2.45.1 2.71.64.72 1.03 1.63 1.03 2.75 0 3.93-2.34 4.8-4.57 5.05.36.32.68.94.68 1.9 0 1.37-.01 2.47-.01 2.81 0 .27.18.6.69.49A10.03 10.03 0 0 0 22 12.25C22 6.58 17.52 2 12 2Z"
          />
        </svg>
      </a>
    </div>
  </header>
</template>
