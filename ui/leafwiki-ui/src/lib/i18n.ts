import i18next from 'i18next'
import { initReactI18next } from 'react-i18next'
// Side-effect: keeps date-fns / Intl locale in sync with the active language.
import './dateLocale'

// Eagerly globs every namespace file under src/locales/<lang>/<namespace>.json.
// Adding a new language only requires a new locale folder — no code change here.
const localeModules = import.meta.glob<{ default: Record<string, unknown> }>(
  '../locales/*/*.json',
  { eager: true },
)

const LOCALE_PATH_PATTERN = /\.\.\/locales\/([^/]+)\/([^/]+)\.json$/

const resources: Record<string, Record<string, Record<string, unknown>>> = {}

for (const path in localeModules) {
  const match = LOCALE_PATH_PATTERN.exec(path)
  if (!match) continue
  const [, lang, namespace] = match
  resources[lang] ??= {}
  resources[lang][namespace] = localeModules[path].default
}

export type AvailableLanguage = {
  code: string
  name: string
}

function selfNameOf(lang: string): string {
  const language = resources[lang]?.common?.language as
    { selfName?: string } | undefined
  return language?.selfName ?? lang
}

export function getAvailableLanguages(): AvailableLanguage[] {
  return Object.keys(resources)
    .sort()
    .map((code) => ({ code, name: selfNameOf(code) }))
}

i18next.use(initReactI18next).init({
  lng: 'en',
  fallbackLng: 'en',
  resources,
  interpolation: {
    escapeValue: false,
  },
})

export default i18next
