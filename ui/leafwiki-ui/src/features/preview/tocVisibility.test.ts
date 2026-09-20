import { describe, expect, it } from 'vitest'
import { shouldShowToc } from './tocVisibility'

describe('shouldShowToc', () => {
  it.each([
    [0, false, false],
    [1, false, false],
    [3, false, false],
    [4, false, true],
    [0, true, false],
    [1, true, true],
    [3, true, true],
    [4, true, true],
  ])(
    'entryCount=%i alwaysShow=%s -> %s',
    (entryCount, alwaysShow, expected) => {
      expect(shouldShowToc(entryCount, alwaysShow)).toBe(expected)
    },
  )
})
