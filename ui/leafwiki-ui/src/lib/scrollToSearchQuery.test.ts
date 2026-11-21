import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  findSearchQueryRange,
  scrollToSearchQuery,
  SEARCH_QUERY_HIGHLIGHT,
} from './scrollToSearchQuery'

const CONTENT_HTML =
  '<p>Hello world</p><p>Find the keyword here please</p><p>keyword again</p>'

// jsdom implements neither the CSS Custom Highlight API nor layout, so the
// registry, Highlight, Range rects and Element.scrollTo are stubbed.
class FakeHighlight {
  readonly ranges: Range[]
  constructor(...ranges: Range[]) {
    this.ranges = ranges
  }
}

let highlights: Map<string, FakeHighlight>
let scrollTo: ReturnType<typeof vi.fn>

function content(): HTMLElement {
  return document.querySelector('.page-viewer__content') as HTMLElement
}

function highlightedTexts(): string[] {
  const highlight = highlights.get(SEARCH_QUERY_HIGHLIGHT)
  return highlight ? highlight.ranges.map((range) => range.toString()) : []
}

beforeEach(() => {
  const sc = document.createElement('div')
  sc.id = 'scroll-container'
  document.body.appendChild(sc)

  const article = document.createElement('article')
  article.className = 'page-viewer__content'
  article.innerHTML = CONTENT_HTML
  document.body.appendChild(article)

  highlights = new Map()
  vi.stubGlobal('CSS', { highlights })
  vi.stubGlobal('Highlight', FakeHighlight)
  // The match sits 400px below the container's top edge and is 20px tall.
  Range.prototype.getBoundingClientRect = () => new DOMRect(0, 400, 50, 20)
  scrollTo = vi.fn()
  Element.prototype.scrollTo = scrollTo as unknown as Element['scrollTo']
})

afterEach(() => {
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('findSearchQueryRange', () => {
  it('covers the first case-insensitive match', () => {
    const range = findSearchQueryRange(content(), 'Keyword')

    expect(range?.toString()).toBe('keyword')
    const firstMatchParagraph = content().querySelectorAll('p')[1]
    expect(firstMatchParagraph.contains(range!.startContainer)).toBe(true)
  })

  it('returns null when no match exists', () => {
    expect(findSearchQueryRange(content(), 'missing')).toBeNull()
  })
})

describe('scrollToSearchQuery', () => {
  it('highlights and scrolls to the first match without changing the DOM', () => {
    scrollToSearchQuery('Keyword', { waitForStableLayout: false })

    expect(highlightedTexts()).toEqual(['keyword'])
    // jsdom's container has no height, so centring puts the match's middle
    // (400 + 20 / 2) at the container's top.
    expect(scrollTo).toHaveBeenCalledWith({ top: 410, behavior: 'smooth' })
    expect(content().innerHTML).toBe(CONTENT_HTML)
  })

  it.each([
    ['a query shorter than 3 characters', 'ky'],
    ['a query without a match', 'missing'],
  ])('does nothing for %s', (_name, query) => {
    scrollToSearchQuery(query, { waitForStableLayout: false })

    expect(highlights.has(SEARCH_QUERY_HIGHLIGHT)).toBe(false)
    expect(scrollTo).not.toHaveBeenCalled()
  })

  it('replaces the previous highlight with the new match', () => {
    scrollToSearchQuery('keyword', { waitForStableLayout: false })
    scrollToSearchQuery('Hello', { waitForStableLayout: false })

    expect(highlightedTexts()).toEqual(['Hello'])
  })

  it('cleanup removes the highlight', () => {
    const cancel = scrollToSearchQuery('keyword', {
      waitForStableLayout: false,
    })
    expect(highlightedTexts()).toEqual(['keyword'])

    cancel()

    expect(highlights.has(SEARCH_QUERY_HIGHLIGHT)).toBe(false)
    expect(content().innerHTML).toBe(CONTENT_HTML)
  })

  it('cleanup leaves a newer highlight alone', () => {
    const cancelFirst = scrollToSearchQuery('keyword', {
      waitForStableLayout: false,
    })
    scrollToSearchQuery('Hello', { waitForStableLayout: false })

    cancelFirst()

    expect(highlightedTexts()).toEqual(['Hello'])
  })

  it('only scrolls when the browser lacks CSS custom highlights', () => {
    vi.stubGlobal('CSS', {})

    const cancel = scrollToSearchQuery('keyword', {
      waitForStableLayout: false,
    })

    expect(scrollTo).toHaveBeenCalled()
    expect(content().innerHTML).toBe(CONTENT_HTML)
    expect(cancel).not.toThrow()
  })
})
