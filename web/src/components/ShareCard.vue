<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { copyText } from '@/lib/clipboard'
import { formatDateTime } from '@/lib/format'
import { useToast } from '@/composables/useToast'
import QrCode from './QrCode.vue'

const props = defineProps<{
  url: string
  passwordRequired: boolean
  expiresAt: string
}>()

const { t, locale } = useI18n()
const { push } = useToast()

const copied = ref(false)
let copiedTimer: number | undefined

const expires = computed(() =>
  t('send.share.expires', { time: formatDateTime(props.expiresAt, locale.value) }),
)

async function copy(): Promise<void> {
  const ok = await copyText(props.url)
  if (!ok) return
  copied.value = true
  push(t('common.copied'))
  window.clearTimeout(copiedTimer)
  copiedTimer = window.setTimeout(() => {
    copied.value = false
  }, 2000)
}

onBeforeUnmount(() => window.clearTimeout(copiedTimer))
</script>

<template>
  <section class="surface p-5 sm:p-6">
    <div class="grid gap-6 md:grid-cols-[minmax(0,1fr)_auto] md:items-start">
      <div class="min-w-0 space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <span v-if="passwordRequired" class="eyebrow border-falafel-700/20">
            <svg viewBox="0 0 20 20" class="h-3 w-3" fill="none" aria-hidden="true">
              <rect x="4" y="9" width="12" height="8" rx="2" stroke="currentColor" stroke-width="1.6" />
              <path d="M7 9V6.5a3 3 0 0 1 6 0V9" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
            </svg>
            {{ t('send.share.passwordBadge') }}
          </span>
          <span class="text-[0.7rem] font-medium text-ink-soft">{{ expires }}</span>
        </div>

        <div>
          <label class="card-label mb-2 block" for="share-url">{{ t('send.share.linkLabel') }}</label>
          <div
            class="flex flex-col gap-2 rounded-2xl border border-falafel-900/10 bg-white/70 p-1.5 transition focus-within:border-falafel-500 focus-within:ring-4 focus-within:ring-falafel-400/15 sm:flex-row sm:items-stretch"
          >
            <input
              id="share-url"
              :value="url"
              readonly
              dir="ltr"
              class="min-w-0 flex-1 bg-transparent px-3 py-2 text-left font-mono text-xs text-ink outline-none select-all"
              @focus="($event.target as HTMLInputElement).select()"
            />
            <button
              type="button"
              class="btn-primary w-full justify-center rounded-xl px-3.5 py-2 text-xs sm:w-auto sm:shrink-0"
              @click="copy"
            >
              <svg
                v-if="!copied"
                viewBox="0 0 20 20"
                class="h-4 w-4"
                fill="none"
                aria-hidden="true"
              >
                <rect x="7" y="7" width="9" height="10" rx="2" stroke="currentColor" stroke-width="1.5" />
                <path d="M13 7V5.5A1.5 1.5 0 0 0 11.5 4h-7A1.5 1.5 0 0 0 3 5.5v8A1.5 1.5 0 0 0 4.5 15H7" stroke="currentColor" stroke-width="1.5" />
              </svg>
              <svg v-else viewBox="0 0 20 20" class="h-4 w-4" fill="none" aria-hidden="true">
                <path d="m4 10.5 4 4 8-9" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
              <span>{{ copied ? t('common.copied') : t('send.share.copy') }}</span>
            </button>
          </div>
        </div>

        <div class="flex items-start gap-2 rounded-2xl bg-falafel-100/60 px-3.5 py-3 text-xs text-falafel-900">
          <svg viewBox="0 0 20 20" class="mt-0.5 h-4 w-4 shrink-0" fill="none" aria-hidden="true">
            <path d="M10 3.5 17 17H3L10 3.5Z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" />
            <path d="M10 8v4m0 2.2v.3" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
          </svg>
          <p class="leading-relaxed">{{ t('send.share.keepOpen') }}</p>
        </div>
      </div>

      <div class="flex flex-col items-center gap-2 md:pt-1">
        <QrCode :value="url" :label="t('send.share.qr')" />
      </div>
    </div>
  </section>
</template>
