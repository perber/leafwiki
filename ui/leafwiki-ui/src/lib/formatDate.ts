import { format as formatWithPattern, formatDistanceToNow } from 'date-fns'
import { enUS } from 'date-fns/locale'
import { useUserSettingsStore } from '@/stores/userSettings'
import { bcp47 } from './dateLocale'

/**
 * Relative time such as "3 days ago" / "vor 3 Tagen". The locale follows the
 * active UI language via date-fns' global default options (see dateLocale.ts).
 */
export function formatRelativeTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (isNaN(date.getTime())) return ''
  return formatDistanceToNow(date, { addSuffix: true })
}

export type DateTimeFormatPrefs = {
  dateFormat: string
  timeFormat: string
}

// date-fns patterns for the explicit presets; anything not listed here
// (i.e. "locale") falls back to Intl in the active UI language.
const DATE_PATTERNS: Record<string, string> = {
  iso: 'yyyy-MM-dd',
  dmy_dot: 'dd.MM.yyyy',
  mdy_slash: 'MM/dd/yyyy',
  dmy_slash: 'dd/MM/yyyy',
}

const TIME_PATTERNS: Record<string, string> = {
  '24h': 'HH:mm',
  '12h': 'h:mm a',
}

function currentPrefs(): DateTimeFormatPrefs {
  const s = useUserSettingsStore.getState()
  return { dateFormat: s.dateFormat, timeFormat: s.timeFormat }
}

function toDate(value?: string): Date | null {
  if (!value) return null
  const date = new Date(value)
  return isNaN(date.getTime()) ? null : date
}

function datePart(
  date: Date,
  lng: string | undefined,
  dateFormat: string,
): string {
  const pattern = DATE_PATTERNS[dateFormat]
  if (pattern) return formatWithPattern(date, pattern)
  return new Intl.DateTimeFormat(bcp47(lng), { dateStyle: 'medium' }).format(
    date,
  )
}

function timePart(
  date: Date,
  lng: string | undefined,
  timeFormat: string,
): string {
  const pattern = TIME_PATTERNS[timeFormat]
  if (pattern) {
    // Pin the 12-hour marker to English AM/PM — date-fns' global default
    // locale would otherwise localise "a" (e.g. "nachm." under a German UI),
    // diverging from the picker label. 24h has no locale-sensitive token.
    const options = timeFormat === '12h' ? { locale: enUS } : undefined
    return formatWithPattern(date, pattern, options)
  }
  return new Intl.DateTimeFormat(bcp47(lng), { timeStyle: 'short' }).format(
    date,
  )
}

/** Absolute date + time. Honours the user's date/time format preference. */
export function formatDateTime(
  value?: string,
  lng?: string,
  prefs: DateTimeFormatPrefs = currentPrefs(),
): string {
  const date = toDate(value)
  if (!date) return ''
  if (prefs.dateFormat === 'locale' && prefs.timeFormat === 'locale') {
    return new Intl.DateTimeFormat(bcp47(lng), {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(date)
  }
  return `${datePart(date, lng, prefs.dateFormat)} ${timePart(date, lng, prefs.timeFormat)}`
}

/** Absolute date only. Honours the user's date format preference. */
export function formatDateOnly(
  value?: string,
  lng?: string,
  prefs: DateTimeFormatPrefs = currentPrefs(),
): string {
  const date = toDate(value)
  if (!date) return ''
  return datePart(date, lng, prefs.dateFormat)
}

/** Absolute time only. Honours the user's time format preference. */
export function formatTimeOnly(
  value?: string,
  lng?: string,
  prefs: DateTimeFormatPrefs = currentPrefs(),
): string {
  const date = toDate(value)
  if (!date) return ''
  return timePart(date, lng, prefs.timeFormat)
}
