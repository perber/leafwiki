import i18next from '@/lib/i18n'
import { afterEach, describe, expect, it } from 'vitest'

// The summary chips in PageHistoryContent count assets; they must read
// "1 asset changed", not "1 asset changes".
afterEach(async () => {
  await i18next.changeLanguage('en')
})

describe('history summary chips', () => {
  it.each([
    ['en', 'chips.assetChangesCount', 1, '1 asset changed'],
    ['en', 'chips.assetChangesCount', 2, '2 assets changed'],
    ['en', 'chips.assetsCount', 1, '1 asset'],
    ['en', 'chips.assetsCount', 2, '2 assets'],
    ['de', 'chips.assetChangesCount', 1, '1 Dateiänderung'],
    ['de', 'chips.assetChangesCount', 2, '2 Dateiänderungen'],
    ['de', 'chips.assetsCount', 1, '1 Datei'],
    ['de', 'chips.assetsCount', 2, '2 Dateien'],
  ])('%s %s with count %i reads %j', async (lng, key, count, expected) => {
    await i18next.changeLanguage(lng)
    expect(i18next.t(key, { ns: 'history', count })).toBe(expected)
  })
})
