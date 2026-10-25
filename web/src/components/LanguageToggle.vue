<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { setLocale, type Locale } from '@/i18n'

const { t, locale } = useI18n()

const options: Array<{ value: Locale; label: string }> = [
  { value: 'en', label: 'EN' },
  { value: 'fa', label: 'فا' },
]

function choose(value: Locale): void {
  if (locale.value !== value) setLocale(value)
}
</script>

<template>
  <div
    class="flex items-center rounded-full border border-falafel-900/10 bg-white/60 p-0.5"
    role="group"
    :aria-label="t('nav.language')"
  >
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      class="cursor-pointer rounded-full px-2.5 py-1 text-[0.7rem] font-semibold transition-colors duration-200"
      :class="
        locale === option.value
          ? 'bg-falafel-500 text-falafel-950 shadow-sm'
          : 'text-ink-soft hover:text-falafel-900'
      "
      :aria-pressed="locale === option.value"
      @click="choose(option.value)"
    >
      {{ option.label }}
    </button>
  </div>
</template>
