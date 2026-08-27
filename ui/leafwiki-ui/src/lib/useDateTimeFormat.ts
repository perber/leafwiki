// Hook wrapper around the locale-aware formatters in ./formatDate so React
// components re-render when the UI language changes (useTranslation subscribes
// the component to languageChanged).

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

  return useMemo(
    () => ({
      formatDateTime: (value?: string) => formatDateTime(value, lng),
      formatDateOnly: (value?: string) => formatDateOnly(value, lng),
      formatTimeOnly: (value?: string) => formatTimeOnly(value, lng),
      formatRelativeTime,
    }),
    [lng],
  )
}
