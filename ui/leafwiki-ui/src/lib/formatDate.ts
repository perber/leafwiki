import { formatDistanceToNow } from 'date-fns'
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

function formatWith(
  value: string | undefined,
  lng: string | undefined,
  options: Intl.DateTimeFormatOptions,
): string {
  if (!value) return ''
  const date = new Date(value)
  if (isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(bcp47(lng), options).format(date)
}

/** Absolute date + time, e.g. "27 Aug 2026, 14:30". Locale-aware. */
export function formatDateTime(value?: string, lng?: string): string {
  return formatWith(value, lng, { dateStyle: 'medium', timeStyle: 'short' })
}

/** Absolute date only, e.g. "27 Aug 2026". Locale-aware. */
export function formatDateOnly(value?: string, lng?: string): string {
  return formatWith(value, lng, { dateStyle: 'medium' })
}

/** Absolute time only, e.g. "14:30". Locale-aware. */
export function formatTimeOnly(value?: string, lng?: string): string {
  return formatWith(value, lng, { timeStyle: 'short' })
}
