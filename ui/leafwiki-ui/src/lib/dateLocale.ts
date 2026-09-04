// Keeps date/time formatting in sync with the active i18next UI language.
//
// - date-fns renders relative times ("3 days ago" / "vor 3 Tagen"); it has no
//   notion of our UI language on its own, so we push the matching locale into
//   its global default options whenever the language changes.
// - Intl.DateTimeFormat needs a BCP-47 tag; `bcp47()` maps our short language
//   codes ('en', 'de') to one.
//
// Imported for its side effect from src/lib/i18n.ts so the listener is wired
// up right after i18next.init().

import { setDefaultOptions } from 'date-fns'
import { de, enUS, type Locale } from 'date-fns/locale'
import i18next from 'i18next'

const DATE_FNS_LOCALES: Record<string, Locale> = {
  en: enUS,
  de,
}

const BCP47_TAGS: Record<string, string> = {
  en: 'en-US',
  de: 'de-DE',
}

function shortCode(lng?: string): string {
  return (lng ?? i18next.language ?? 'en').split('-')[0]
}

/** date-fns Locale object for the given (or active) UI language. */
export function dateFnsLocale(lng?: string): Locale {
  return DATE_FNS_LOCALES[shortCode(lng)] ?? enUS
}

/** BCP-47 tag for Intl.* for the given (or active) UI language. */
export function bcp47(lng?: string): string {
  const raw = lng ?? i18next.language ?? 'en'
  return BCP47_TAGS[shortCode(raw)] ?? raw
}

function applyDateFnsLocale(lng?: string): void {
  setDefaultOptions({ locale: dateFnsLocale(lng) })
}

applyDateFnsLocale()
i18next.on('languageChanged', applyDateFnsLocale)
