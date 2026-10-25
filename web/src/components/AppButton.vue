<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    loading?: boolean
    disabled?: boolean
    variant?: 'primary' | 'ghost' | 'quiet'
    type?: 'button' | 'submit'
    block?: boolean
  }>(),
  {
    loading: false,
    disabled: false,
    variant: 'primary',
    type: 'button',
    block: false,
  },
)

const classes = computed(() => [
  props.variant === 'primary' ? 'btn-primary' : props.variant === 'ghost' ? 'btn-ghost' : 'btn-quiet',
  props.block ? 'w-full' : '',
])
</script>

<template>
  <button
    :type="type"
    :class="classes"
    :disabled="disabled || loading"
    :aria-busy="loading"
  >
    <svg
      v-if="loading"
      class="h-4 w-4 animate-spin"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-opacity="0.25" stroke-width="3" />
      <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" stroke-width="3" stroke-linecap="round" />
    </svg>
    <slot />
  </button>
</template>
