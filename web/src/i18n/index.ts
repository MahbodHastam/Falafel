import { createI18n } from 'vue-i18n'
import en from './en'
import fa from './fa'

export const SUPPORTED_LOCALES = ['en', 'fa'] as const
export type Locale = (typeof SUPPORTED_LOCALES)[number]

const STORAGE_KEY = 'falafel.locale'

function initialLocale(): Locale {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'en' || saved === 'fa') return saved
  const browser = navigator.language.toLowerCase()
  return browser.startsWith('fa') ? 'fa' : 'en'
}

export const i18n = createI18n({
  legacy: false,
  locale: initialLocale(),
  fallbackLocale: 'en',
  messages: { en, fa },
})

export function currentLocale(): Locale {
  return i18n.global.locale.value as Locale
}

export function setLocale(locale: Locale): void {
  i18n.global.locale.value = locale
  localStorage.setItem(STORAGE_KEY, locale)
  applyDocumentLocale(locale)
}

export function applyDocumentLocale(locale: Locale): void {
  document.documentElement.lang = locale
  document.documentElement.dir = locale === 'fa' ? 'rtl' : 'ltr'
}

applyDocumentLocale(initialLocale())
