import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  type MockInstance,
  vi,
} from 'vitest'
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

// jsdom has neither method; vi.spyOn needs them on the prototype first, and
// restoreAllMocks puts these placeholders back after each test.
Range.prototype.getBoundingClientRect = () => new DOMRect()
Element.prototype.scrollTo = () => {}

let highlights: Map<string, FakeHighlight>
let scrollTo: MockInstance

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
  vi.spyOn(Range.prototype, 'getBoundingClientRect').mockReturnValue(
    new DOMRect(0, 400, 50, 20),
  )
  scrollTo = vi
    .spyOn(Element.prototype, 'scrollTo')
    .mockImplementation(() => {})
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

  it.each([
    // A match only exists per text node; one split by inline markup is not found.
    [
      'across an element boundary',
      '<p>foo <strong>bar</strong></p>',
      'foo bar',
      null,
    ],
    [
      'inside a code block',
      '<pre><code>let needle = 1</code></pre>',
      'needle',
      'needle',
    ],
    [
      'inside a collapsed details element',
      '<details><summary>s</summary><p>hidden needle</p></details>',
      'needle',
      'needle',
    ],
    ['in a script', '<script>needle</script><p>x</p>', 'needle', null],
    ['with Unicode in the query', '<p>Grüße aus Köln</p>', 'KÖLN', 'Köln'],
    // Lowercasing İ (U+0130) yields two UTF-16 units; offsets must not shift.
    ['after a dotted capital I', '<p>İab xyz</p>', 'xyz', 'xyz'],
    ['right after a dotted capital I', '<p>İstanbul</p>', 'stan', 'stan'],
    ['with a final sigma', '<p>ΟΔΟΣ</p>', 'οδος', 'ΟΔΟΣ'],
    [
      'with regex characters in the query',
      '<p>use a+b (c)</p>',
      'A+B (C)',
      'a+b (c)',
    ],
  ])('handles a match %s', (_name, html, query, expected) => {
    const root = document.createElement('div')
    root.innerHTML = html

    expect(findSearchQueryRange(root, query)?.toString() ?? null).toBe(expected)
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

  it('centres the match in a scrolled container', () => {
    const container = document.getElementById('scroll-container')!
    vi.spyOn(container, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(0, 100, 800, 600),
    )
    vi.spyOn(container, 'clientHeight', 'get').mockReturnValue(600)
    vi.spyOn(container, 'scrollTop', 'get').mockReturnValue(250)
    vi.spyOn(Range.prototype, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(0, 900, 50, 20),
    )

    scrollToSearchQuery('keyword', {
      behavior: 'auto',
      waitForStableLayout: false,
    })

    // The match is 800px into the visible area; centring leaves
    // (600 - 20) / 2 = 290px above it: 250 + 800 - 290.
    expect(scrollTo).toHaveBeenCalledWith({ top: 760, behavior: 'auto' })
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

  it('cleanup does not throw after the content re-rendered under the highlight', () => {
    const cancel = scrollToSearchQuery('keyword', {
      waitForStableLayout: false,
    })
    // React replaces the text nodes the range points into.
    content().innerHTML = '<p>Different page</p>'

    expect(cancel).not.toThrow()
    expect(highlights.has(SEARCH_QUERY_HIGHLIGHT)).toBe(false)
  })

  it('cancelling before the layout settles neither highlights nor scrolls', () => {
    vi.useFakeTimers()
    try {
      const cancelFirst = scrollToSearchQuery('keyword')
      cancelFirst()
      const cancelSecond = scrollToSearchQuery('Hello')
      cancelSecond()
      scrollToSearchQuery('again')

      vi.runAllTimers()

      expect(highlightedTexts()).toEqual(['again'])
      expect(scrollTo).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
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
