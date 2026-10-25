const UNITS = ['B', 'KB', 'MB', 'GB', 'TB']

export function formatBytes(bytes: number, locale = 'en'): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return `0 ${UNITS[0]}`
  }
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < UNITS.length - 1) {
    value /= 1024
    unit += 1
  }
  // One decimal below 10 units, so 1.5 MB does not read as 2 MB.
  const digits = value < 10 && unit > 0 ? 1 : 0
  const formatted = new Intl.NumberFormat(locale, {
    maximumFractionDigits: digits,
    minimumFractionDigits: 0,
  }).format(value)
  return `${formatted} ${UNITS[unit]}`
}

export function formatSpeed(bytesPerSecond: number, locale = 'en'): string {
  return `${formatBytes(bytesPerSecond, locale)}/s`
}

type Translate = (key: string, values?: Record<string, unknown>) => string

export function formatDuration(ms: number, locale = 'en', t: Translate): string {
  // Under 0.9 s the rounded value would be one second, so say less than a second.
  if (!Number.isFinite(ms) || ms < 900) return t('common.lessThanSecond')
  const total = Math.round(ms / 1000)
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  const number = new Intl.NumberFormat(locale)
  if (minutes === 0) return t('common.seconds', { n: number.format(seconds) })
  return t('common.minutesSeconds', {
    m: number.format(minutes),
    s: number.format(seconds),
  })
}

export function formatPercent(fraction: number, locale = 'en'): string {
  const clamped = Math.min(1, Math.max(0, fraction))
  return new Intl.NumberFormat(locale, {
    style: 'percent',
    maximumFractionDigits: clamped < 0.1 ? 1 : 0,
  }).format(clamped)
}

export function formatTime(iso: string, locale = 'en'): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(locale, {
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

export function formatDateTime(iso: string, locale = 'en'): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  const sameDay = date.toDateString() === new Date().toDateString()
  // Today's timestamps need only the time; older ones also need the date.
  return new Intl.DateTimeFormat(locale, {
    ...(sameDay ? {} : { month: 'short', day: 'numeric' }),
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}
