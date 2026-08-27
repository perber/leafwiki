// Hook wrapper around the locale-aware formatters in ./formatDate so React
// components re-render when the UI language OR the user's date/time format
// preference changes (useTranslation subscribes the component to
// languageChanged; the store selectors subscribe it to preference changes).

import { useUserSettingsStore } from '@/stores/userSettings'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  formatDateOnly,
  formatDateTime,
  formatRelativeTime,
  formatTimeOnly,
} from './formatDate'

export type DateTimeFormatters = {
  formatDateTime: (value?: string) => string
  formatDateOnly: (value?: string) => string
  formatTimeOnly: (value?: string) => string
  formatRelativeTime: (value?: string) => string
}

export function useDateTimeFormat(): DateTimeFormatters {
  const { i18n } = useTranslation()
  const lng = i18n.language
  const dateFormat = useUserSettingsStore((s) => s.dateFormat)
  const timeFormat = useUserSettingsStore((s) => s.timeFormat)

  return useMemo(() => {
    const prefs = { dateFormat, timeFormat }
    return {
      formatDateTime: (value?: string) => formatDateTime(value, lng, prefs),
      formatDateOnly: (value?: string) => formatDateOnly(value, lng, prefs),
      formatTimeOnly: (value?: string) => formatTimeOnly(value, lng, prefs),
      formatRelativeTime,
    }
  }, [lng, dateFormat, timeFormat])
}
