// remark-flexible-markers renders `==text==` and `=r=text==` as
// <mark class="flexible-marker …">. Search-query marks
// (lib/scrollToSearchQuery.ts) and raw HTML <mark> don't carry that class.
const HIGHLIGHT_SELECTOR = 'mark.flexible-marker'

export type Highlight = {
  text: string
  element: Element
}

// Reads the highlights out of rendered preview output, in document order.
// Empty markers (`====`) have nothing to show and are skipped.
export function collectHighlights(root: ParentNode): Highlight[] {
  return Array.from(root.querySelectorAll(HIGHLIGHT_SELECTOR))
    .map((element) => ({
      text: (element.textContent ?? '').replace(/\s+/g, ' ').trim(),
      element,
    }))
    .filter((highlight) => highlight.text.length > 0)
}
