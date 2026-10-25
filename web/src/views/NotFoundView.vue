<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <div class="mx-auto flex max-w-lg flex-col items-center gap-6 py-8 text-center sm:py-14">
    <svg
      viewBox="0 0 180 180"
      class="h-40 w-40 drop-shadow-[0_24px_40px_rgba(140,72,32,0.18)]"
      aria-hidden="true"
    >
      <defs>
        <radialGradient id="nf-ball" cx="34%" cy="28%" r="85%">
          <stop offset="0%" stop-color="#fadf9f" />
          <stop offset="48%" stop-color="#f29f05" />
          <stop offset="100%" stop-color="#bf5b04" />
        </radialGradient>
        <mask id="nf-bite">
          <rect width="180" height="180" fill="white" />
          <circle cx="165" cy="24" r="44" fill="black" />
          <circle cx="144" cy="4" r="27" fill="black" />
        </mask>
      </defs>
      <ellipse cx="90" cy="172" rx="54" ry="6" fill="#592c1c" opacity="0.08" />
      <g mask="url(#nf-bite)">
        <circle cx="90" cy="94" r="70" fill="url(#nf-ball)" />
        <g fill="#8c4820" opacity="0.5">
          <circle cx="68" cy="58" r="4.4" />
          <circle cx="102" cy="50" r="3.2" />
          <circle cx="122" cy="78" r="4.6" />
          <circle cx="112" cy="116" r="3.6" />
          <circle cx="72" cy="128" r="4.6" />
          <circle cx="48" cy="96" r="3.1" />
          <circle cx="90" cy="88" r="5" />
        </g>
        <path
          d="M90 30a64 64 0 0 1 60 43"
          fill="none"
          stroke="#fdeccf"
          stroke-opacity="0.6"
          stroke-width="7"
          stroke-linecap="round"
        />
      </g>
      <circle cx="129" cy="42" r="4" fill="#bf5b04" opacity="0.7" />
      <circle cx="140" cy="58" r="2.8" fill="#bf5b04" opacity="0.55" />
      <circle cx="118" cy="58" r="2.2" fill="#f29f05" opacity="0.8" />
    </svg>

    <div class="space-y-3">
      <p class="text-xs font-bold uppercase tracking-[0.3em] text-falafel-700">
        {{ t('notFound.eyebrow') }}
      </p>
      <h1 class="text-balance text-3xl font-extrabold tracking-tight sm:text-4xl">
        {{ t('notFound.title') }}
      </h1>
      <p class="mx-auto max-w-sm text-sm leading-relaxed text-ink-soft">
        {{ t('notFound.body') }}
      </p>
    </div>

    <RouterLink to="/" class="btn-primary">{{ t('notFound.cta') }}</RouterLink>
  </div>
</template>
