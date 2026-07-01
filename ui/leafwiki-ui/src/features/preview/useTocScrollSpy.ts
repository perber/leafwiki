import { useEffect, useState } from 'react'
import type { TocEntry } from './extractTocEntries'

export function useTocScrollSpy(entries: TocEntry[]): string | null {
  const [activeId, setActiveId] = useState<string | null>(null)

  useEffect(() => {
    if (entries.length === 0) return
    const scrollContainer = document.getElementById('scroll-container')
    if (!scrollContainer) return

    const updateActive = () => {
      const containerRect = scrollContainer.getBoundingClientRect()
      const triggerY = containerRect.top + 120

      // Near bottom: short paragraphs may prevent lower headings from ever
      // crossing triggerY, so also activate headings visible in the viewport.
      const atBottom =
        Math.abs(
          scrollContainer.scrollHeight -
            scrollContainer.scrollTop -
            scrollContainer.clientHeight,
        ) < 5

      let active = entries[0].id
      for (const entry of entries) {
        const el = document.getElementById(entry.id)
        if (!el) continue
        const top = el.getBoundingClientRect().top
        if (top <= triggerY || (atBottom && top < containerRect.bottom)) {
          active = entry.id
        }
      }
      setActiveId(active)
    }

    updateActive()
    scrollContainer.addEventListener('scroll', updateActive, { passive: true })
    return () => scrollContainer.removeEventListener('scroll', updateActive)
  }, [entries])

  return activeId
}
