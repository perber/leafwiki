import { describe, expect, it } from 'vitest'
import { shouldReserveTocPane } from './tocPaneReserve'

describe('shouldReserveTocPane', () => {
  it('always reserves the desktop TOC column (Safari breadcrumb alignment, #1549)', () => {
    expect(shouldReserveTocPane()).toBe(true)
  })
})
