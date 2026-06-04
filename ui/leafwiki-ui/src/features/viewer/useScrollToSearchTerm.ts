import { getNavigationVisitKey } from '@/lib/navigationVisit'
import { waitUntilHeightStabilizes } from '@/lib/scrollToHeadline'
import { getNavigationSearchQuery } from '@/lib/searchNavigationState'
import { useEffect, useRef } from 'react'
import { useLocation } from 'react-router-dom'

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
  const visitKey = getNavigationVisitKey(location)
  const highlightedRef = useRef(new Set<string>())

  useEffect(() => {
    if (isLoading || !content || !searchQuery || location.hash) return

    const highlightKey = `${visitKey}:${searchQuery}`
    if (highlightedRef.current.has(highlightKey)) return

    const scrollContainer = document.getElementById('scroll-container')
    if (!scrollContainer) return

    const cancel = waitUntilHeightStabilizes(scrollContainer, () => {
      highlightAndScroll(searchQuery)
      highlightedRef.current.add(highlightKey)
    })

    return () => {
      cancel()
      const contentEl = document.querySelector('.page-viewer__content')
      if (contentEl) cleanupHighlights(contentEl)
    }
  }, [content, isLoading, searchQuery, location.hash, visitKey])
}
