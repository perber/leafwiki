import { formatDistanceToNow, type Locale } from 'date-fns'
import { enUS, ru } from 'date-fns/locale'

import i18next from './i18n'

const dateFnsLocales: Record<string, Locale> = {
  en: enUS,
  ru,
}

function getDateFnsLocale(): Locale {
  const lang = (i18next.resolvedLanguage ?? i18next.language).split('-')[0]
  return dateFnsLocales[lang] ?? enUS
}

export function formatRelativeTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (isNaN(date.getTime())) return ''
  return formatDistanceToNow(date, {
    addSuffix: true,
    locale: getDateFnsLocale(),
  })
}
