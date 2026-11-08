import { PAGE_CONTENT_SELECTOR } from '@/lib/pageContentRoot'
import { useEffect, useMemo, useState } from 'react'
import { collectHighlights } from '../preview/collectHighlights'
import type { PagePanelSection } from '../preview/pagePanelSection'

type Options = {
  content?: string
  isLoading?: boolean
}

function contentRoot(): Element | null {
  return document.querySelector(PAGE_CONTENT_SELECTOR)
}

// A mark inside a closed collapsible (`:::collapsed` → <details>) has no
// layout box to scroll to, so every <details> between it and the content
// root is opened first.
function openEnclosingDetails(element: Element, root: Element) {
  for (
    let details = element.parentElement?.closest('details');
    details && root.contains(details);
    details = details.parentElement?.closest('details')
  ) {
    details.open = true
  }
}

function scrollToHighlight(index: number) {
  const root = contentRoot()
  if (!root) return
  // Resolved at click time: the preview may have re-rendered its marks since
  // the list was built. The marks carry no ids on purpose, generated ids
  // could collide with the page's own ids (sanitizer runs with clobberPrefix '').
  const element = collectHighlights(root)[index]?.element
  if (!element) return
  openEnclosingDetails(element, root)
  element.scrollIntoView({ behavior: 'smooth', block: 'center' })
}

// Builds the "Highlights" section from the rendered page (`==text==`). The
// preview renders synchronously, so the marks are in the DOM once the effect
// runs after the commit that shows the new content.
export function useHighlightsSection({
  content,
  isLoading,
}: Options): PagePanelSection {
  const [labels, setLabels] = useState<string[]>([])

  useEffect(() => {
    const root = isLoading || !content ? null : contentRoot()
    setLabels(root ? collectHighlights(root).map((h) => h.text) : [])
  }, [content, isLoading])

  return useMemo(
    () => ({
      id: 'highlights',
      titleKey: 'toc.highlights',
      items: labels.map((label, index) => ({
        kind: 'anchor' as const,
        key: `highlight-${index}`,
        label,
        onSelect: () => scrollToHighlight(index),
        testId: `highlight-${index}`,
      })),
    }),
    [labels],
  )
}
