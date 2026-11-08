import { afterEach, describe, expect, it } from 'vitest'
import i18next from './i18n'

describe('document lang', () => {
  afterEach(async () => {
    await i18next.changeLanguage('en')
  })

  it('starts with the active UI language', () => {
    expect(document.documentElement.lang).toBe(i18next.language)
  })

  it('follows language changes', async () => {
    await i18next.changeLanguage('de')
    expect(document.documentElement.lang).toBe('de')

    await i18next.changeLanguage('en')
    expect(document.documentElement.lang).toBe('en')
  })
})
