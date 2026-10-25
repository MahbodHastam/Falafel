<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    value: number
    active?: boolean
    height?: 'sm' | 'md'
  }>(),
  { active: false, height: 'md' },
)

const percent = computed(() => Math.round(Math.min(1, Math.max(0, props.value)) * 100))
</script>

<template>
  <div
    class="relative w-full overflow-hidden rounded-full bg-falafel-900/10"
    :class="height === 'md' ? 'h-2.5' : 'h-1.5'"
    role="progressbar"
    :aria-valuenow="percent"
    aria-valuemin="0"
    aria-valuemax="100"
  >
    <div
      class="relative h-full overflow-hidden rounded-full bg-gradient-to-r from-falafel-500 via-falafel-600 to-falafel-700 transition-[width] duration-300 ease-out"
      :class="active ? 'progress-sheen' : ''"
      :style="{ width: `${percent}%` }"
    />
  </div>
</template>
