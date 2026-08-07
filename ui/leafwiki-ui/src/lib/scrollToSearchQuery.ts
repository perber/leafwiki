const HIGHLIGHT_CLASS = 'search-query-highlight'
const CONTENT_SELECTOR = '.page-viewer__content'
const SCROLL_CONTAINER_ID = 'scroll-container'

export type ScrollToSearchQueryOptions = {
  behavior?: ScrollBehavior
  waitForStableLayout?: boolean
  rootSelector?: string
}

export function clearSearchQueryHighlights(
  root: ParentNode = document,
): void {
  root.querySelectorAll(`mark.${HIGHLIGHT_CLASS}`).forEach((mark) => {
    const parent = mark.parentNode
    if (!parent) return
    while (mark.firstChild) {
      parent.insertBefore(mark.firstChild, mark)
    }
    parent.removeChild(mark)
    parent.normalize()
  })
}

function findFirstTextMatch(
  root: HTMLElement,
  query: string,
): { node: Text; index: number } | null {
  const normalizedQuery = query.toLowerCase()
  if (!normalizedQuery) return null

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
    const text = current.textContent ?? ''
    const index = text.toLowerCase().indexOf(normalizedQuery)
    if (index >= 0) {
      return { node: current as Text, index }
    }
    current = walker.nextNode()
  }

  return null
}

function wrapMatch(node: Text, index: number, length: number): HTMLElement {
  const fullText = node.textContent ?? ''
  const before = fullText.slice(0, index)
  const matchText = fullText.slice(index, index + length)
  const after = fullText.slice(index + length)

  const mark = document.createElement('mark')
  mark.className = HIGHLIGHT_CLASS
  mark.textContent = matchText
  mark.setAttribute('data-testid', 'search-query-highlight')

  const parent = node.parentNode
  if (!parent) return mark

  const fragment = document.createDocumentFragment()
  if (before) fragment.appendChild(document.createTextNode(before))
  fragment.appendChild(mark)
  if (after) fragment.appendChild(document.createTextNode(after))
  parent.replaceChild(fragment, node)
  return mark
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
 * wraps it in a highlight mark, and scrolls it into view.
 */
export function scrollToSearchQuery(
  query: string,
  {
    behavior = 'smooth',
    waitForStableLayout = true,
    rootSelector = CONTENT_SELECTOR,
  }: ScrollToSearchQueryOptions = {},
): () => void {
  const trimmed = query.trim()
  if (trimmed.length < 3) return () => {}

  const scrollContainer = document.getElementById(
    SCROLL_CONTAINER_ID,
  ) as HTMLElement | null
  if (!scrollContainer) return () => {}

  let cancelled = false

  const run = () => {
    if (cancelled) return

    const contentRoot = document.querySelector(
      rootSelector,
    ) as HTMLElement | null
    if (!contentRoot) return

    clearSearchQueryHighlights(contentRoot)

    const match = findFirstTextMatch(contentRoot, trimmed)
    if (!match) return

    const mark = wrapMatch(match.node, match.index, trimmed.length)
    mark.scrollIntoView({ behavior, block: 'center', inline: 'nearest' })
  }

  if (waitForStableLayout) {
    waitUntilHeightStabilizes(scrollContainer, run)
  } else {
    run()
  }

  return () => {
    cancelled = true
    const contentRoot = document.querySelector(rootSelector)
    if (contentRoot) clearSearchQueryHighlights(contentRoot)
  }
}
