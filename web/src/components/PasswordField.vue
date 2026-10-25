<script setup lang="ts">
import { ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'

defineProps<{
  modelValue: string
  placeholder: string
  minLength?: number
  autocomplete?: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const { t } = useI18n()
const id = useId()
const visible = ref(false)

function onInput(event: Event): void {
  emit('update:modelValue', (event.target as HTMLInputElement).value)
}
</script>

<template>
  <div class="relative">
    <label :for="id" class="sr-only">{{ placeholder }}</label>
    <input
      :id="id"
      :type="visible ? 'text' : 'password'"
      :value="modelValue"
      :placeholder="placeholder"
      :minlength="minLength"
      :autocomplete="autocomplete ?? 'off'"
      spellcheck="false"
      class="field pe-20"
      @input="onInput"
    />
    <button
      type="button"
      class="absolute inset-y-0 end-1 my-auto flex h-9 cursor-pointer items-center gap-1 rounded-xl px-3 text-[0.7rem] font-semibold text-ink-soft transition-colors hover:bg-falafel-900/5 hover:text-falafel-900"
      :aria-label="visible ? t('common.hide') : t('common.show')"
      @click="visible = !visible"
    >
      <svg v-if="visible" viewBox="0 0 24 24" class="h-4 w-4" fill="none" aria-hidden="true">
        <path d="M3 3l18 18M10.6 10.7a2 2 0 0 0 2.8 2.8" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
        <path d="M6.7 6.8C4.6 8.2 3 10.3 2.2 12c1.4 3 4.4 5.2 8 5.2 1.3 0 2.5-.3 3.6-.8M9.9 5.1A9.4 9.4 0 0 1 12 4.9c3.9 0 7 2.5 8.2 5.5-.4.8-1 1.6-1.7 2.2" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
      </svg>
      <svg v-else viewBox="0 0 24 24" class="h-4 w-4" fill="none" aria-hidden="true">
        <path d="M2.5 12C3.8 8.9 7 6.8 10.6 6.8S17.4 8.9 18.7 12c-1.3 3.1-4.5 5.2-8.1 5.2S3.8 15.1 2.5 12Z" stroke="currentColor" stroke-width="1.7" />
        <circle cx="10.6" cy="12" r="2.4" stroke="currentColor" stroke-width="1.7" />
      </svg>
      <span>{{ visible ? t('common.hide') : t('common.show') }}</span>
    </button>
  </div>
</template>
