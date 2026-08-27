import i18next from '@/lib/i18n'
import { afterEach, describe, expect, it } from 'vitest'
import { bcp47 } from './dateLocale'
import {
  formatDateOnly,
  formatDateTime,
  formatRelativeTime,
  formatTimeOnly,
} from './formatDate'

const ISO = '2026-08-27T14:30:00.000Z'

afterEach(async () => {
  await i18next.changeLanguage('en')
})

describe('bcp47', () => {
  it('maps short UI language codes to full locale tags', () => {
    expect(bcp47('en')).toBe('en-US')
    expect(bcp47('de')).toBe('de-DE')
  })

  it('falls back to the raw value for unknown codes', () => {
    expect(bcp47('fr')).toBe('fr')
  })
})

describe('absolute formatters', () => {
  it('return an empty string for missing or invalid input', () => {
    expect(formatDateTime(undefined)).toBe('')
    expect(formatDateOnly('')).toBe('')
    expect(formatTimeOnly('not-a-date')).toBe('')
  })

  it('format the same instant differently per locale', () => {
    const en = formatDateOnly(ISO, 'en')
    const de = formatDateOnly(ISO, 'de')
    expect(en).not.toBe('')
    expect(de).not.toBe('')
    expect(en).not.toBe(de)
    // German medium date uses a trailing dot on the day number
    expect(de).toMatch(/^27\./)
  })

  it('formatDateTime includes both a date and a time component', () => {
    const out = formatDateTime(ISO, 'en')
    expect(out).toMatch(/2026/)
    expect(out).toMatch(/\d{1,2}:\d{2}/)
  })
})

describe('explicit format presets', () => {
  it('formatDateOnly honours each date preset', () => {
    expect(
      formatDateOnly(ISO, 'en', { dateFormat: 'iso', timeFormat: 'locale' }),
    ).toBe('2026-08-27')
    expect(
      formatDateOnly(ISO, 'en', {
        dateFormat: 'dmy_dot',
        timeFormat: 'locale',
      }),
    ).toBe('27.08.2026')
    expect(
      formatDateOnly(ISO, 'en', {
        dateFormat: 'mdy_slash',
        timeFormat: 'locale',
      }),
    ).toBe('08/27/2026')
    expect(
      formatDateOnly(ISO, 'en', {
        dateFormat: 'dmy_slash',
        timeFormat: 'locale',
      }),
    ).toBe('27/08/2026')
  })

  it('formatTimeOnly honours 24h and 12h presets (shape is timezone-stable)', () => {
    expect(
      formatTimeOnly(ISO, 'en', { dateFormat: 'locale', timeFormat: '24h' }),
    ).toMatch(/^\d{2}:\d{2}$/)
    expect(
      formatTimeOnly(ISO, 'en', { dateFormat: 'locale', timeFormat: '12h' }),
    ).toMatch(/^\d{2}:\d{2}\s?\S+$/)
  })

  it('formatDateTime combines the chosen date and time presets', () => {
    expect(
      formatDateTime(ISO, 'en', { dateFormat: 'iso', timeFormat: '24h' }),
    ).toMatch(/^2026-08-27 \d{2}:\d{2}$/)
  })

  it('"locale" for both keeps the single combined Intl format', () => {
    const out = formatDateTime(ISO, 'de', {
      dateFormat: 'locale',
      timeFormat: 'locale',
    })
    expect(out).toMatch(/27\./)
    expect(out).toMatch(/\d{1,2}:\d{2}/)
  })
})

describe('formatRelativeTime locale', () => {
  it('follows the active i18next language', async () => {
    const past = new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString()

    await i18next.changeLanguage('en')
    expect(formatRelativeTime(past)).toMatch(/ago/)

    await i18next.changeLanguage('de')
    expect(formatRelativeTime(past)).toMatch(/vor/)
  })

  it('returns an empty string for missing or invalid input', () => {
    expect(formatRelativeTime(undefined)).toBe('')
    expect(formatRelativeTime('nope')).toBe('')
  })
})
