import { renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { TocEntry } from './extractTocEntries'
import { useHeadingsSection } from './useHeadingsSection'

const scrollMock = vi.fn()
vi.mock('@/lib/scrollToHeadline', () => ({
  scrollToHeadlineHash: (...args: unknown[]) => scrollMock(...args),
}))

const entries: TocEntry[] = [
  { level: 1, text: 'Introduction', id: 'introduction' },
  { level: 2, text: 'Hello World', id: 'hello world' },
]

let scrollContainer: HTMLDivElement

beforeEach(() => {
  vi.clearAllMocks()
  scrollContainer = document.createElement('div')
  scrollContainer.id = 'scroll-container'
  vi.spyOn(scrollContainer, 'getBoundingClientRect').mockReturnValue({
    top: 0,
  } as DOMRect)
  document.body.appendChild(scrollContainer)
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('useHeadingsSection', () => {
  it('maps each heading to an anchor item titled "on this page"', () => {
    const { result } = renderHook(() => useHeadingsSection(entries))

    expect(result.current.id).toBe('headings')
    expect(result.current.titleKey).toBe('toc.onThisPage')
    expect(result.current.items).toEqual([
      expect.objectContaining({
        kind: 'anchor',
        key: 'introduction',
        label: 'Introduction',
        level: 1,
        testId: 'entry-introduction',
      }),
      expect.objectContaining({
        kind: 'anchor',
        key: 'hello world',
        label: 'Hello World',
        level: 2,
        testId: 'entry-hello world',
      }),
    ])
  })

  it('marks the heading picked by the scroll spy as active', () => {
    const { result } = renderHook(() => useHeadingsSection(entries))

    // No heading elements in the DOM → the spy defaults to the first entry.
    expect(
      result.current.items.map((item) => item.kind === 'anchor' && item.active),
    ).toEqual([true, false])
  })

  it('scrolls to the encoded heading id when an item is selected', () => {
    const { result } = renderHook(() => useHeadingsSection(entries))
    const item = result.current.items[1]

    if (item.kind !== 'anchor') throw new Error('expected an anchor item')
    item.onSelect?.()

    expect(scrollMock).toHaveBeenCalledWith(
      `#${encodeURIComponent('hello world')}`,
      { waitForStableLayout: false },
    )
  })

  it('leaves items without onSelect when not clickable', () => {
    const { result } = renderHook(() =>
      useHeadingsSection(entries, { clickable: false }),
    )

    for (const item of result.current.items) {
      if (item.kind !== 'anchor') throw new Error('expected an anchor item')
      expect(item.onSelect).toBeUndefined()
    }
  })

  it('returns an empty section and no scroll listener without headings', () => {
    const spy = vi.spyOn(scrollContainer, 'addEventListener')
    const { result } = renderHook(() => useHeadingsSection([]))

    expect(result.current.items).toEqual([])
    expect(spy).not.toHaveBeenCalled()
  })
})
