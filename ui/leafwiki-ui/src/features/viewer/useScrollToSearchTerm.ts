import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'

const SEARCH_QUERY_STATE_KEY = 'leafwikiSearchQuery'

export function getNavigationSearchQuery(state: unknown): string | undefined {
  if (typeof state === 'object' && state !== null) {
    const s = state as Record<string, unknown>
    const q = s[SEARCH_QUERY_STATE_KEY]
    if (typeof q === 'string' && q.length > 0) return q
  }
  return undefined
}

function cleanupHighlights(root: Element) {
  root.querySelectorAll('mark.search-highlight').forEach((el) => {
    const parent = el.parentNode
    if (!parent) return
    parent.replaceChild(document.createTextNode(el.textContent ?? ''), el)
    parent.normalize()
  })
}

function highlightAndScroll(searchTerm: string) {
  const contentEl = document.querySelector('.page-viewer__content')
  if (!contentEl) return

  cleanupHighlights(contentEl)

  const walker = document.createTreeWalker(contentEl, NodeFilter.SHOW_TEXT)
  const term = searchTerm.toLowerCase()

  let node: Node | null
  while ((node = walker.nextNode())) {
    const text = node.textContent ?? ''
    const idx = text.toLowerCase().indexOf(term)
    if (idx === -1) continue

    const parent = node.parentNode
    if (!parent) continue

    const mark = document.createElement('mark')
    mark.className = 'search-highlight'
    mark.textContent = text.slice(idx, idx + searchTerm.length)

    const fragment = document.createDocumentFragment()
    const before = text.slice(0, idx)
    const after = text.slice(idx + searchTerm.length)
    if (before) fragment.appendChild(document.createTextNode(before))
    fragment.appendChild(mark)
    if (after) fragment.appendChild(document.createTextNode(after))

    parent.replaceChild(fragment, node)
    mark.scrollIntoView({ behavior: 'smooth', block: 'center' })
    return
  }
}

type Options = {
  content?: string
  isLoading?: boolean
}

export function useScrollToSearchTerm({ content, isLoading }: Options) {
  const location = useLocation()
  const searchQuery = getNavigationSearchQuery(location.state)

  useEffect(() => {
    if (isLoading || !content || !searchQuery || location.hash) return

    const scrollContainer = document.getElementById('scroll-container')
    if (!scrollContainer) return

    const timeout = setTimeout(() => {
      highlightAndScroll(searchQuery)
    }, 500)

    return () => {
      clearTimeout(timeout)
      const contentEl = document.querySelector('.page-viewer__content')
      if (contentEl) cleanupHighlights(contentEl)
    }
  }, [content, isLoading, searchQuery, location.hash])
}
