<script setup lang="ts">
import { ref, watch } from 'vue'
import QRCode from 'qrcode'

const props = defineProps<{ value: string; label: string }>()

const dataURL = ref('')

watch(
  () => props.value,
  async (value) => {
    if (!value) {
      dataURL.value = ''
      return
    }
    try {
      dataURL.value = await QRCode.toDataURL(value, {
        width: 320,
        margin: 1,
        errorCorrectionLevel: 'M',
        color: { dark: '#592c1c', light: '#fff9f0' },
      })
    } catch {
      dataURL.value = ''
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="flex flex-col items-center gap-2">
    <div
      class="grid h-[9.5rem] w-[9.5rem] place-items-center overflow-hidden rounded-2xl border border-falafel-900/10 bg-falafel-50 p-2"
    >
      <img
        v-if="dataURL"
        :src="dataURL"
        :alt="label"
        class="h-full w-full rounded-xl object-contain"
        width="152"
        height="152"
      />
      <div v-else class="h-full w-full animate-pulse rounded-xl bg-falafel-100" />
    </div>
  </div>
</template>
