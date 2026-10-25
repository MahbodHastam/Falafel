import { useI18n } from 'vue-i18n'

/**
 * Turns a server error code such as `room_busy` into a translated sentence.
 * Unknown codes fall back to a generic message.
 */
export function useErrorMessage() {
  const { t, te } = useI18n()

  return function errorMessage(code: string): string {
    const key = `errors.${code}`
    return te(key) ? t(key) : t('errors.generic')
  }
}
