<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ name: string }>()

// Show up to four characters of the extension, or "FILE" when there is none.
const extension = computed(() => {
  const dot = props.name.lastIndexOf('.')
  if (dot <= 0 || dot === props.name.length - 1) return 'FILE'
  return props.name.slice(dot + 1).toUpperCase().slice(0, 4)
})
</script>

<template>
  <span class="grid shrink-0 place-items-center rounded-2xl font-extrabold tracking-wide">
    {{ extension }}
  </span>
</template>
