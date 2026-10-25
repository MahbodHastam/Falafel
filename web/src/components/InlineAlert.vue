<script setup lang="ts">
defineProps<{ message: string }>()
</script>

<template>
  <p
    class="flex items-start gap-2 rounded-2xl bg-red-50 px-3.5 py-3 text-xs font-medium text-red-800 ring-1 ring-red-700/15"
    role="alert"
  >
    <svg viewBox="0 0 20 20" class="mt-0.5 h-4 w-4 shrink-0" fill="none" aria-hidden="true">
      <circle cx="10" cy="10" r="7.5" stroke="currentColor" stroke-width="1.5" />
      <path d="M10 6v5m0 2.4v.2" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
    </svg>
    {{ message }}
  </p>
</template>
