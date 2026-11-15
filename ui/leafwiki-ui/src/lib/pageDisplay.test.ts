import { describe, expect, it } from 'vitest'
import { DEFAULT_PAGE_DISPLAY, parsePageDisplay } from './pageDisplay'

describe('parsePageDisplay', () => {
  it.each([
    { name: 'undefined', raw: undefined },
    { name: 'null', raw: null },
    { name: 'a string', raw: 'wide' },
    { name: 'an array', raw: [true, 'wide'] },
    { name: 'an empty object (older server)', raw: {} },
  ])('returns the defaults for $name', ({ raw }) => {
    expect(parsePageDisplay(raw)).toEqual(DEFAULT_PAGE_DISPLAY)
  })

  it('keeps valid values', () => {
    expect(
      parsePageDisplay({ alwaysShowToc: true, contentWidth: 'wide' }),
    ).toEqual({ alwaysShowToc: true, contentWidth: 'wide' })
  })

  it('ignores fields that are not display settings', () => {
    expect(
      parsePageDisplay({
        alwaysShowToc: true,
        contentWidth: 'wide',
        publicAccess: true,
      }),
    ).toEqual({ alwaysShowToc: true, contentWidth: 'wide' })
  })

  it.each([
    { name: 'a string flag', alwaysShowToc: 'true' },
    { name: 'a number flag', alwaysShowToc: 1 },
    { name: 'a null flag', alwaysShowToc: null },
  ])('treats $name as false', ({ alwaysShowToc }) => {
    expect(parsePageDisplay({ alwaysShowToc }).alwaysShowToc).toBe(false)
  })

  it.each([
    { name: 'an unknown width', contentWidth: 'huge' },
    // Same rule as the backend (exact match, no case folding or trimming).
    { name: 'a capitalised width', contentWidth: 'Wide' },
    { name: 'a non-string width', contentWidth: 1 },
    { name: 'an Object.prototype key', contentWidth: 'constructor' },
  ])('falls back to the default width for $name', ({ contentWidth }) => {
    expect(parsePageDisplay({ contentWidth }).contentWidth).toBe('default')
  })
})
