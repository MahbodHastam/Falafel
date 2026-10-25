<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import AppButton from '@/components/AppButton.vue'
import PasswordField from '@/components/PasswordField.vue'

defineProps<{
  passwordRequired: boolean
  passwordError: string
  starting: boolean
  largeFile: boolean
}>()

const emit = defineEmits<{ start: [] }>()
const password = defineModel<string>('password', { required: true })

const { t } = useI18n()
</script>

<template>
  <div class="space-y-4">
    <div v-if="passwordRequired" class="space-y-2">
      <div class="flex items-center gap-2">
        <span class="grid h-8 w-8 place-items-center rounded-xl bg-falafel-500/15 text-falafel-800">
          <svg viewBox="0 0 20 20" class="h-4 w-4" fill="none" aria-hidden="true">
            <rect x="4" y="9" width="12" height="8" rx="2" stroke="currentColor" stroke-width="1.6" />
            <path d="M7 9V6.5a3 3 0 0 1 6 0V9" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
          </svg>
        </span>
        <div>
          <h2 class="text-sm font-bold text-ink">{{ t('download.password.title') }}</h2>
          <p class="text-[0.7rem] text-ink-soft">{{ t('download.password.subtitle') }}</p>
        </div>
      </div>
      <PasswordField
        v-model="password"
        :placeholder="t('download.password.placeholder')"
        autocomplete="current-password"
      />
      <Transition name="soft">
        <p v-if="passwordError" class="px-1 text-xs font-medium text-red-700" role="alert">
          {{ passwordError }}
        </p>
      </Transition>
    </div>

    <AppButton block :loading="starting" :disabled="starting" @click="emit('start')">
      <svg v-if="!starting" viewBox="0 0 20 20" class="h-4 w-4" fill="none" aria-hidden="true">
        <path d="M10 3v10m0 0 4-4m-4 4-4-4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
        <path d="M4 15.5h12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
      </svg>
      {{ starting ? t('download.starting') : t('download.start') }}
    </AppButton>

    <p class="text-center text-[0.68rem] text-ink-soft">{{ t('download.keepOpen') }}</p>
    <p v-if="largeFile" class="text-center text-[0.65rem] text-ink-soft/80">
      {{ t('download.largeFileHint') }}
    </p>
  </div>
</template>
