import { describe, expect, it } from 'vitest'
import { getAvailableLanguages } from './i18n'
import de from '../locales/de/common.json'
import en from '../locales/en/common.json'

const localeModules = import.meta.glob<{ default: Record<string, unknown> }>(
  '../locales/*/*.json',
  { eager: true },
)

const NAMESPACES_BY_LANG: Record<string, Record<string, unknown>> = {}
for (const path in localeModules) {
  const match = /\.\.\/locales\/([^/]+)\/([^/]+)\.json$/.exec(path)
  if (!match) continue
  const [, lang, namespace] = match
  NAMESPACES_BY_LANG[lang] ??= {}
  NAMESPACES_BY_LANG[lang][namespace] = localeModules[path].default
}

function collectKeyPaths(
  value: unknown,
  prefix: string,
  keys: Set<string>,
): void {
  if (typeof value !== 'object' || value === null) {
    keys.add(prefix)
    return
  }
  for (const [key, child] of Object.entries(value)) {
    collectKeyPaths(child, prefix ? `${prefix}.${key}` : key, keys)
  }
}

describe('getAvailableLanguages', () => {
  it('includes English and German with their self-name translated', () => {
    const languages = getAvailableLanguages()

    expect(languages).toContainEqual({ code: 'en', name: 'English' })
    expect(languages).toContainEqual({ code: 'de', name: 'Deutsch' })
  })

  it('reflects language.selfName from each language common.json', () => {
    expect(en.language.selfName).toBe('English')
    expect(de.language.selfName).toBe('Deutsch')
  })
})

describe('locale namespace key parity', () => {
  const languages = Object.keys(NAMESPACES_BY_LANG)
  const referenceLang = 'en'

  it('every discovered language ships the same namespace files as English', () => {
    const referenceNamespaces = Object.keys(
      NAMESPACES_BY_LANG[referenceLang],
    ).sort()
    for (const lang of languages) {
      if (lang === referenceLang) continue
      expect(Object.keys(NAMESPACES_BY_LANG[lang]).sort()).toEqual(
        referenceNamespaces,
      )
    }
  })

  it.each(Object.keys(NAMESPACES_BY_LANG[referenceLang]))(
    'namespace "%s" has identical keys across all languages',
    (namespace) => {
      const referenceKeys = new Set<string>()
      collectKeyPaths(
        NAMESPACES_BY_LANG[referenceLang][namespace],
        '',
        referenceKeys,
      )

      for (const lang of languages) {
        if (lang === referenceLang) continue
        const keys = new Set<string>()
        collectKeyPaths(NAMESPACES_BY_LANG[lang][namespace], '', keys)
        expect(
          [...keys].sort(),
          `locales/${lang}/${namespace}.json is missing or has extra keys compared to English`,
        ).toEqual([...referenceKeys].sort())
      }
    },
  )
})
