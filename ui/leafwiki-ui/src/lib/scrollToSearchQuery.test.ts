import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  clearSearchQueryHighlights,
  scrollToSearchQuery,
} from './scrollToSearchQuery'

beforeEach(() => {
  const sc = document.createElement('div')
  sc.id = 'scroll-container'
  document.body.appendChild(sc)

  const content = document.createElement('article')
  content.className = 'page-viewer__content'
  content.innerHTML =
    '<p>Hello world</p><p>Find the keyword here please</p><p>keyword again</p>'
  document.body.appendChild(content)
})

afterEach(() => {
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

describe('scrollToSearchQuery', () => {
  it('highlights and scrolls to the first case-insensitive match', () => {
    const scrollSpy = vi
      .spyOn(HTMLElement.prototype, 'scrollIntoView')
      .mockImplementation(() => {})

    scrollToSearchQuery('Keyword', { waitForStableLayout: false })

    const mark = document.querySelector(
      'mark.search-query-highlight',
    ) as HTMLElement
    expect(mark).not.toBeNull()
    expect(mark.textContent).toBe('keyword')
    expect(scrollSpy).toHaveBeenCalled()

    const content = document.querySelector(
      '.page-viewer__content',
    ) as HTMLElement
    const firstMatchParagraph = content.querySelectorAll('p')[1]
    expect(firstMatchParagraph.contains(mark)).toBe(true)
  })

  it('does nothing for queries shorter than 3 characters', () => {
    scrollToSearchQuery('ky', { waitForStableLayout: false })
    expect(document.querySelector('mark.search-query-highlight')).toBeNull()
  })

  it('does nothing when no match exists', () => {
    scrollToSearchQuery('missing', { waitForStableLayout: false })
    expect(document.querySelector('mark.search-query-highlight')).toBeNull()
  })

  it('clears previous highlights before applying a new match', () => {
    scrollToSearchQuery('keyword', { waitForStableLayout: false })
    expect(
      document.querySelectorAll('mark.search-query-highlight'),
    ).toHaveLength(1)

    scrollToSearchQuery('Hello', { waitForStableLayout: false })
    const marks = document.querySelectorAll('mark.search-query-highlight')
    expect(marks).toHaveLength(1)
    expect(marks[0].textContent).toBe('Hello')
  })

  it('cleanup removes highlights', () => {
    const cancel = scrollToSearchQuery('keyword', {
      waitForStableLayout: false,
    })
    expect(document.querySelector('mark.search-query-highlight')).not.toBeNull()
    cancel()
    expect(document.querySelector('mark.search-query-highlight')).toBeNull()
  })
})

describe('clearSearchQueryHighlights', () => {
  it('unwraps highlight marks without losing text', () => {
    scrollToSearchQuery('keyword', { waitForStableLayout: false })
    clearSearchQueryHighlights()
    expect(document.querySelector('mark.search-query-highlight')).toBeNull()
    expect(document.body.textContent).toContain('Find the keyword here please')
  })
})
