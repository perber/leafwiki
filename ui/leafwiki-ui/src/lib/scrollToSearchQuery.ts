import { PAGE_CONTENT_SELECTOR } from './pageContentRoot'

// Name of the CSS custom highlight (styled via ::highlight() in index.css).
export const SEARCH_QUERY_HIGHLIGHT = 'leafwiki-search-query'
const SCROLL_CONTAINER_ID = 'scroll-container'

export type ScrollToSearchQueryOptions = {
  behavior?: ScrollBehavior
  waitForStableLayout?: boolean
  rootSelector?: string
}

// Matches case-insensitively on the original text, so the offsets are valid
// for a Range on the text node. Lowercasing first shifts them: "İ" (U+0130)
// lowercases to two UTF-16 units.
function findFirstTextMatch(
  root: HTMLElement,
  query: string,
): { node: Text; index: number; length: number } | null {
  if (!query) return null
  const pattern = new RegExp(query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'iu')

  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(node) {
      const parent = node.parentElement
      if (!parent) return NodeFilter.FILTER_REJECT
      if (parent.closest('script, style, noscript')) {
        return NodeFilter.FILTER_REJECT
      }
      if (!node.textContent?.trim()) return NodeFilter.FILTER_REJECT
      return NodeFilter.FILTER_ACCEPT
    },
  })

  let current = walker.nextNode()
  while (current) {
    const match = pattern.exec(current.textContent ?? '')
    if (match) {
      return {
        node: current as Text,
        index: match.index,
        length: match[0].length,
      }
    }
    current = walker.nextNode()
  }

  return null
}

// Returns a Range over the first case-insensitive occurrence of `query` in
// the text under `root`, or null.
export function findSearchQueryRange(
  root: HTMLElement,
  query: string,
): Range | null {
  const match = findFirstTextMatch(root, query)
  if (!match) return null
  const end = match.index + match.length
  // Range.setEnd throws IndexSizeError past the node's end.
  if (match.length === 0 || end > match.node.length) return null
  const range = document.createRange()
  range.setStart(match.node, match.index)
  range.setEnd(match.node, end)
  return range
}

// The CSS Custom Highlight API (Chrome/Edge 105+, Safari 17.2+, Firefox 140+)
// paints the match without touching the DOM. Wrapping the match in a <mark>
// replaced a text node React still owned, so the next reconcile of the page
// content threw NotFoundError (insertBefore/removeChild) and tripped the
// preview error boundary.
function canHighlight(): boolean {
  return (
    typeof CSS !== 'undefined' &&
    'highlights' in CSS &&
    typeof Highlight === 'function'
  )
}

function scrollRangeIntoView(
  range: Range,
  scrollContainer: HTMLElement,
  behavior: ScrollBehavior,
) {
  const rangeRect = range.getBoundingClientRect()
  const containerRect = scrollContainer.getBoundingClientRect()
  const offset =
    rangeRect.top -
    containerRect.top -
    (scrollContainer.clientHeight - rangeRect.height) / 2
  scrollContainer.scrollTo({
    top: scrollContainer.scrollTop + offset,
    behavior,
  })
}

function waitUntilHeightStabilizes(
  element: HTMLElement,
  callback: () => void,
  interval = 250,
  maxTotalTime = 3000,
  stableTime = 500,
) {
  let lastHeight = element.scrollHeight
  let stableFor = 0
  let elapsedTime = 0

  const checkHeight = () => {
    const currentHeight = element.scrollHeight
    if (currentHeight === lastHeight) {
      stableFor += interval
      if (stableFor >= stableTime) {
        callback()
        return
      }
    } else {
      lastHeight = currentHeight
      stableFor = 0
    }
    elapsedTime += interval
    if (elapsedTime < maxTotalTime) {
      setTimeout(checkHeight, interval)
    } else {
      callback()
    }
  }

  setTimeout(checkHeight, interval)
}

/**
 * Finds the first case-insensitive occurrence of `query` in the page content,
 * highlights it (where the browser supports CSS custom highlights), and
 * scrolls it into view.
 */
export function scrollToSearchQuery(
  query: string,
  {
    behavior = 'smooth',
    waitForStableLayout = true,
    rootSelector = PAGE_CONTENT_SELECTOR,
  }: ScrollToSearchQueryOptions = {},
): () => void {
  const trimmed = query.trim()
  if (trimmed.length < 3) return () => {}

  const scrollContainer = document.getElementById(
    SCROLL_CONTAINER_ID,
  ) as HTMLElement | null
  if (!scrollContainer) return () => {}

  let cancelled = false
  let highlight: Highlight | null = null

  const run = () => {
    if (cancelled) return

    const contentRoot = document.querySelector(
      rootSelector,
    ) as HTMLElement | null
    if (!contentRoot) return

    const range = findSearchQueryRange(contentRoot, trimmed)
    if (!range) return

    if (canHighlight()) {
      highlight = new Highlight(range)
      CSS.highlights.set(SEARCH_QUERY_HIGHLIGHT, highlight)
    }
    scrollRangeIntoView(range, scrollContainer, behavior)
  }

  if (waitForStableLayout) {
    waitUntilHeightStabilizes(scrollContainer, run)
  } else {
    run()
  }

  return () => {
    cancelled = true
    // Only remove our own entry; a newer call may have replaced it.
    if (highlight && CSS.highlights.get(SEARCH_QUERY_HIGHLIGHT) === highlight) {
      CSS.highlights.delete(SEARCH_QUERY_HIGHLIGHT)
    }
  }
}
