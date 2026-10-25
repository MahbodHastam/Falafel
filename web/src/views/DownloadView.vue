<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

import DownloadDone from '@/components/download/DownloadDone.vue'
import DownloadError from '@/components/download/DownloadError.vue'
import DownloadProgress from '@/components/download/DownloadProgress.vue'
import DownloadReady from '@/components/download/DownloadReady.vue'
import FileBadge from '@/components/FileBadge.vue'
import ShieldNote from '@/components/ShieldNote.vue'
import NotFoundView from '@/views/NotFoundView.vue'
import { useDownloadSession } from '@/composables/useDownloadSession'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { formatBytes, formatDuration, formatPercent, formatSpeed } from '@/lib/format'

// Files above this size are written straight to disk when the browser can.
const LARGE_FILE_BYTES = 256 * 1024 * 1024

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const token = typeof route.params.token === 'string' ? route.params.token : ''
const { phase, room, errorCode, retryAfter, bytes, progress, speed, etaMs, elapsedMs, averageSpeed, savedName, size, load, start, cancel, retry } =
  useDownloadSession(token)

const password = ref('')
const localError = ref('')
const shake = ref(false)
let shakeTimer: number | undefined

const mimeLabel = computed(() => {
  const mime = room.value?.mimeType ?? ''
  const subtype = mime.split('/').pop()?.split(';')[0]?.toUpperCase() ?? ''
  if (!subtype || subtype === 'OCTET-STREAM' || subtype.length > 14) return 'FILE'
  return subtype
})

const percent = computed(() => formatPercent(progress.value, locale.value))
const transferring = computed(
  () => phase.value === 'starting' || phase.value === 'connecting' || phase.value === 'transferring',
)
const canStart = computed(() => phase.value === 'ready' || phase.value === 'password')
const largeFile = computed(() => (room.value?.fileSize ?? 0) >= LARGE_FILE_BYTES)

const statusLabel = computed(() => {
  switch (phase.value) {
    case 'starting':
      return t('download.starting')
    case 'connecting':
      return t('download.connecting')
    case 'transferring':
      return t('download.transferring')
    default:
      return t('download.waiting')
  }
})

const etaLabel = computed(() =>
  etaMs.value > 0 ? t('send.progress.eta', { time: formatDuration(etaMs.value, locale.value, t) }) : '',
)

const passwordError = computed(() => {
  if (phase.value !== 'password') return ''
  if (localError.value) return localError.value
  if (errorCode.value === 'wrong_password') return t('download.wrongPassword')
  if (errorCode.value === 'too_many_attempts') {
    return t('download.attemptsWarning', { seconds: retryAfter.value })
  }
  return ''
})

function triggerShake(): void {
  shake.value = false
  window.clearTimeout(shakeTimer)
  window.requestAnimationFrame(() => {
    shake.value = true
    shakeTimer = window.setTimeout(() => {
      shake.value = false
    }, 500)
  })
}

async function onStart(): Promise<void> {
  localError.value = ''
  if (phase.value === 'password' && !password.value) {
    localError.value = t('download.wrongPassword')
    triggerShake()
    return
  }
  await start(password.value)
}

function onRetry(): void {
  password.value = ''
  localError.value = ''
  retry()
}

function goHome(): void {
  void router.push('/')
}

watch(
  () => [phase.value, errorCode.value],
  () => {
    if (phase.value === 'password' && errorCode.value) triggerShake()
  },
)

watch(password, () => {
  localError.value = ''
})

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="mx-auto w-full max-w-xl">
    <NotFoundView v-if="phase === 'not_found'" />

    <div v-else class="space-y-6">
      <section class="surface p-5 sm:p-6" :class="shake ? 'shake' : ''" aria-live="polite">
        <div v-if="phase === 'loading'" class="space-y-4">
          <div class="h-16 animate-pulse rounded-2xl bg-falafel-900/5" />
          <div class="h-11 animate-pulse rounded-2xl bg-falafel-900/5" />
          <div class="h-12 animate-pulse rounded-2xl bg-falafel-900/5" />
        </div>

        <template v-else>
          <div
            v-if="room"
            class="mb-5 flex items-center gap-3 rounded-2xl bg-falafel-50/80 p-4 ring-1 ring-falafel-900/5"
          >
            <FileBadge
              :name="room.fileName"
              class="h-12 w-12 bg-white text-[0.65rem] font-extrabold text-falafel-800 shadow-sm ring-1 ring-falafel-900/5"
            />
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-semibold text-ink" :title="room.fileName">
                {{ room.fileName }}
              </p>
              <p class="mt-0.5 text-[0.7rem] text-ink-soft">
                {{ t('download.fileInfo', { size: formatBytes(room.fileSize, locale), type: mimeLabel }) }}
              </p>
            </div>
          </div>

          <Transition name="soft" mode="out-in">
            <DownloadReady
              v-if="canStart"
              v-model:password="password"
              :password-required="phase === 'password'"
              :password-error="passwordError"
              :starting="phase === 'starting'"
              :large-file="largeFile"
              @start="onStart"
            />

            <DownloadProgress
              v-else-if="transferring"
              :status="statusLabel"
              :speed="speed > 0 ? formatSpeed(speed, locale) : ''"
              :percent="percent"
              :received="formatBytes(bytes, locale)"
              :total="formatBytes(size, locale)"
              :value="progress"
              :active="phase === 'transferring'"
              :eta="etaLabel"
              @cancel="cancel"
            />

            <DownloadDone
              v-else-if="phase === 'completed'"
              :saved-name="savedName"
              :size="formatBytes(size, locale)"
              :duration="formatDuration(elapsedMs, locale, t)"
              :average-speed="formatSpeed(averageSpeed, locale)"
              @done="goHome"
            />

            <DownloadError
              v-else-if="phase === 'error'"
              :message="errorMessage(errorCode)"
              @retry="onRetry"
            />
          </Transition>
        </template>
      </section>

      <ShieldNote />
    </div>
  </div>
</template>
