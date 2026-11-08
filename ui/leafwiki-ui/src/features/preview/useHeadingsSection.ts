import { scrollToHeadlineHash } from '@/lib/scrollToHeadline'
import { useMemo } from 'react'
import type { TocEntry } from './extractTocEntries'
import type { PagePanelSection } from './pagePanelSection'
import { useTocScrollSpy } from './useTocScrollSpy'

type Options = {
  clickable?: boolean
}

// Builds the "On this page" section. It owns the scroll spy, so a caller that
// feeds one section to several panels runs a single scroll listener.
export function useHeadingsSection(
  entries: TocEntry[],
  { clickable = true }: Options = {},
): PagePanelSection {
  const activeId = useTocScrollSpy(entries)

  return useMemo(
    () => ({
      id: 'headings',
      titleKey: 'toc.onThisPage',
      items: entries.map((entry) => ({
        kind: 'anchor' as const,
        key: entry.id,
        label: entry.text,
        level: entry.level,
        active: activeId === entry.id,
        onSelect: clickable
          ? () =>
              scrollToHeadlineHash(`#${encodeURIComponent(entry.id)}`, {
                waitForStableLayout: false,
              })
          : undefined,
        testId: `entry-${entry.id}`,
      })),
    }),
    [entries, activeId, clickable],
  )
}
