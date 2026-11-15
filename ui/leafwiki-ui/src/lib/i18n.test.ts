import { describe, expect, it } from 'vitest'
import i18next, { getAvailableLanguages } from './i18n'
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

// The subpage count is the first plural key read with { count }; each
// locale's Intl plural rules pick the _one/_other suffix (French treats 0 as
// singular, Chinese has only "other"), so check the resolved text, not just
// the key parity above.
describe('section.subpageCount plural forms', () => {
  it.each([
    { lang: 'en', count: 0, want: '0 subpages' },
    { lang: 'en', count: 1, want: '1 subpage' },
    { lang: 'en', count: 12, want: '12 subpages' },
    { lang: 'de', count: 0, want: '0 Unterseiten' },
    { lang: 'de', count: 1, want: '1 Unterseite' },
    { lang: 'de', count: 12, want: '12 Unterseiten' },
    { lang: 'es', count: 1, want: '1 subpágina' },
    { lang: 'es', count: 12, want: '12 subpáginas' },
    { lang: 'fr', count: 0, want: '0 sous-page' },
    { lang: 'fr', count: 1, want: '1 sous-page' },
    { lang: 'fr', count: 12, want: '12 sous-pages' },
    { lang: 'zh', count: 1, want: '1 个子页面' },
    { lang: 'zh', count: 12, want: '12 个子页面' },
  ])('$lang, $count -> "$want"', ({ lang, count, want }) => {
    const t = i18next.getFixedT(lang, 'viewer')
    expect(t('section.subpageCount', { count })).toBe(want)
  })

  it('covers every shipped locale', () => {
    for (const { code } of getAvailableLanguages()) {
      const t = i18next.getFixedT(code, 'viewer')
      const text = t('section.subpageCount', { count: 3 })
      expect(text, code).toContain('3')
      expect(text, code).not.toContain('section.subpageCount')
    }
  })
})
