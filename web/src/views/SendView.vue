<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import AppButton from '@/components/AppButton.vue'
import FileBadge from '@/components/FileBadge.vue'
import FileDropzone from '@/components/FileDropzone.vue'
import InlineAlert from '@/components/InlineAlert.vue'
import PasswordField from '@/components/PasswordField.vue'
import ShareCard from '@/components/ShareCard.vue'
import ShieldNote from '@/components/ShieldNote.vue'
import StatsPanel from '@/components/StatsPanel.vue'
import { useSendSession } from '@/composables/useSendSession'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { getConfig } from '@/lib/api'
import { formatBytes } from '@/lib/format'

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const {
  phase,
  room,
  shareURL,
  receivedBytes,
  errorCode,
  config,
  active,
  start,
  endSession,
} = useSendSession()

const file = ref<File | null>(null)
const password = ref('')
const protect = ref(false)
const validationError = ref('')
const maxFileSize = ref(10 * 1024 * 1024 * 1024)

const minPassword = computed(() => config.value?.minPasswordLength ?? 4)
const maxLabel = computed(() => formatBytes(config.value?.maxFileSize ?? maxFileSize.value, locale.value))
const fileName = computed(() => file.value?.name ?? '')
const fileSize = computed(() => file.value?.size ?? 0)

const startError = computed(() => {
  if (phase.value === 'failed' && errorCode.value && !room.value) {
    return errorMessage(errorCode.value)
  }
  return validationError.value
})

async function onSelect(selected: File): Promise<void> {
  validationError.value = ''
  const limit = config.value?.maxFileSize ?? maxFileSize.value
  if (selected.size > limit) {
    validationError.value = t('send.file.tooLarge', { size: formatBytes(limit, locale.value) })
    return
  }
  file.value = selected
  errorCode.value = ''
  if (phase.value === 'failed') phase.value = 'idle'
}

function clearFile(): void {
  file.value = null
  validationError.value = ''
}

async function onStart(): Promise<void> {
  validationError.value = ''
  if (!file.value) return
  if (protect.value && password.value.length < minPassword.value) {
    validationError.value = t('errors.password_too_short')
    return
  }
  await start(file.value, protect.value ? password.value : '')
}

async function sendAnother(): Promise<void> {
  await endSession()
  file.value = null
  password.value = ''
  protect.value = false
  validationError.value = ''
}

async function finishSession(): Promise<void> {
  await endSession()
  file.value = null
  password.value = ''
  protect.value = false
}

onMounted(async () => {
  try {
    const loaded = await getConfig()
    config.value = loaded
    maxFileSize.value = loaded.maxFileSize
  } catch {
    // ignore
  }
})
</script>

<template>
  <div class="mx-auto w-full max-w-4xl">
    <Transition name="page" mode="out-in">
      <div v-if="!active" key="compose" class="space-y-8">
        <header class="mx-auto max-w-2xl space-y-4 text-center">
          <span class="eyebrow">
            <svg viewBox="0 0 20 20" class="h-3 w-3" fill="none" aria-hidden="true">
              <circle cx="10" cy="10" r="7" stroke="currentColor" stroke-width="1.5" />
              <path d="M10 3v14M3 10h14" stroke="currentColor" stroke-width="1.2" />
            </svg>
            {{ t('send.eyebrow') }}
          </span>
          <h1 class="text-balance text-4xl font-extrabold leading-[1.04] tracking-tight sm:text-6xl">
            {{ t('send.title') }}
          </h1>
          <p class="mx-auto max-w-xl text-pretty text-sm leading-relaxed text-ink-soft sm:text-base">
            {{ t('send.subtitle') }}
          </p>
        </header>

        <section class="surface mx-auto max-w-2xl p-5 sm:p-7">
          <Transition name="soft" mode="out-in">
            <FileDropzone
              v-if="!file"
              key="drop"
              :title="t('send.drop.title')"
              :subtitle="t('send.drop.subtitle')"
              :hint="t('send.drop.hint', { size: maxLabel })"
              @select="onSelect"
            />
            <div
              v-else
              key="file"
              class="flex items-center gap-3 rounded-3xl border border-falafel-900/10 bg-white/70 p-4 sm:p-5"
            >
              <FileBadge
                :name="fileName"
                class="h-14 w-14 bg-falafel-100 text-[0.7rem] text-falafel-800"
              />
              <div class="min-w-0 flex-1">
                <p class="card-label">{{ t('send.file.ready') }}</p>
                <p class="truncate text-sm font-semibold text-ink" :title="fileName">{{ fileName }}</p>
                <p class="mt-0.5 text-xs tabular-nums text-ink-soft">
                  {{ formatBytes(fileSize, locale) }}
                </p>
              </div>
              <button
                type="button"
                class="grid h-9 w-9 shrink-0 place-items-center rounded-full text-ink-soft transition hover:bg-falafel-900/5 hover:text-falafel-900"
                :aria-label="t('send.file.remove')"
                :title="t('send.file.remove')"
                @click="clearFile"
              >
                <svg viewBox="0 0 20 20" class="h-4 w-4" fill="none" aria-hidden="true">
                  <path d="m5 5 10 10M15 5 5 15" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
                </svg>
              </button>
            </div>
          </Transition>

          <div class="mt-5">
            <button
              type="button"
              class="flex w-full cursor-pointer items-center justify-between gap-3 rounded-2xl px-1 py-1.5 text-start"
              role="switch"
              :aria-checked="protect"
              @click="protect = !protect"
            >
              <span class="flex items-center gap-2.5">
                <span
                  class="grid h-9 w-9 place-items-center rounded-xl transition-colors"
                  :class="protect ? 'bg-falafel-500/15 text-falafel-800' : 'bg-falafel-900/5 text-ink-soft'"
                >
                  <svg viewBox="0 0 20 20" class="h-4 w-4" fill="none" aria-hidden="true">
                    <rect x="4" y="9" width="12" height="8" rx="2" stroke="currentColor" stroke-width="1.6" />
                    <path d="M7 9V6.5a3 3 0 0 1 6 0V9" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
                  </svg>
                </span>
                <span>
                  <span class="block text-sm font-semibold text-ink">{{ t('send.password.toggle') }}</span>
                  <span class="block text-[0.7rem] text-ink-soft">{{ t('common.optional') }}</span>
                </span>
              </span>
              <span
                class="relative h-6 w-11 shrink-0 rounded-full transition-colors duration-300"
                :class="protect ? 'bg-falafel-600' : 'bg-falafel-900/15'"
              >
                <span
                  class="absolute top-0.5 h-5 w-5 rounded-full bg-white shadow-sm dark:bg-falafel-200 transition-all duration-300"
                  :class="protect ? 'start-[1.375rem]' : 'start-0.5'"
                />
              </span>
            </button>

            <div
              class="grid transition-[grid-template-rows] duration-300 ease-out"
              :class="protect ? 'grid-rows-[1fr]' : 'grid-rows-[0fr]'"
            >
              <div class="overflow-hidden">
                <div
                  class="space-y-1.5 pt-3 transition-[opacity,transform,filter] duration-300 ease-out"
                  :class="
                    protect
                      ? 'translate-y-0 opacity-100 blur-0'
                      : '-translate-y-1 opacity-0 blur-[2px]'
                  "
                  :inert="protect ? undefined : true"
                  :aria-hidden="!protect"
                >
                  <PasswordField
                    v-model="password"
                    :placeholder="t('send.password.placeholder')"
                    :min-length="minPassword"
                    autocomplete="new-password"
                  />
                  <p class="px-1 text-[0.68rem] text-ink-soft">{{ t('send.password.hint') }}</p>
                </div>
              </div>
            </div>
          </div>

          <Transition name="soft">
            <InlineAlert v-if="startError" class="mt-4" :message="startError" />
          </Transition>

          <AppButton
            block
            class="mt-5"
            :loading="phase === 'creating'"
            :disabled="!file || phase === 'creating'"
            @click="onStart"
          >
            {{ phase === 'creating' ? t('send.starting') : t('send.start') }}
          </AppButton>

          <ShieldNote class="mt-3" />
        </section>
      </div>

      <div v-else key="share" class="space-y-5">
        <ShareCard
          v-if="room"
          :url="shareURL"
          :password-required="room.passwordRequired"
          :expires-at="room.expiresAt"
        />

        <StatsPanel
          v-if="room"
          :room="room"
          :phase="phase"
          :error-code="errorCode"
          :received-bytes="receivedBytes"
        />

        <Transition name="soft">
          <InlineAlert v-if="phase === 'failed'" :message="errorMessage(errorCode)" />
        </Transition>

        <div class="flex flex-wrap items-center justify-end gap-2">
          <AppButton v-if="phase === 'completed'" variant="primary" @click="sendAnother">
            {{ t('send.another') }}
          </AppButton>
          <AppButton variant="ghost" @click="finishSession">
            {{ t('send.end') }}
          </AppButton>
        </div>
      </div>
    </Transition>
  </div>
</template>
