<script setup lang="ts">
import { watchEffect } from 'vue'
import { useI18n } from 'vue-i18n'

import AppFooter from '@/components/AppFooter.vue'
import AppHeader from '@/components/AppHeader.vue'
import ToastHost from '@/components/ToastHost.vue'

const { t, locale } = useI18n()

watchEffect(() => {
  document.title = t('meta.title')
  document.documentElement.lang = locale.value
  document.documentElement.dir = locale.value === 'fa' ? 'rtl' : 'ltr'
  const description = document.querySelector('meta[name="description"]')
  description?.setAttribute('content', t('meta.description'))
})
</script>

<template>
  <div class="relative flex min-h-dvh flex-col overflow-x-clip">
    <a
      href="#main"
      class="sr-only rounded-full bg-falafel-900 px-4 py-2 text-sm font-semibold text-cream focus:not-sr-only focus:absolute focus:top-3 focus:start-3 focus:z-50"
    >
      {{ t('common.skipToContent') }}
    </a>

    <AppHeader />

    <main id="main" class="mx-auto w-full max-w-5xl flex-1 px-4 pb-16 pt-8 sm:px-6 sm:pt-12">
      <RouterView v-slot="{ Component }">
        <Transition name="page" mode="out-in">
          <component :is="Component" />
        </Transition>
      </RouterView>
    </main>

    <AppFooter />
    <ToastHost />
  </div>
</template>
