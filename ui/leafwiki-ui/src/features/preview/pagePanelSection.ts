// The page side panel and its dropdown render a list of sections built from
// per-page data (headings, attachments, …). A new section is one more entry
// where the sections are composed (features/viewer/PageViewer.tsx); the panels
// only know these item kinds, which are render primitives (ADR-0021).

export type PagePanelItem =
  | {
      kind: 'anchor'
      key: string
      label: string
      // Heading depth, drives the indentation; 1 when omitted.
      level?: number
      active?: boolean
      // Undefined renders the item as a non-interactive entry.
      onSelect?: () => void
      // Suffix only: each panel prefixes its own namespace (`toc-`,
      // `toc-dropdown-`) so both can be addressed in tests.
      testId: string
    }
  | {
      kind: 'download'
      key: string
      label: string
      href: string
      fileName: string
      testId: string
    }

export type PagePanelSection = {
  // Unique across the sections of one panel: the panels use it as React key.
  id: string
  // i18n key in the `viewer` namespace.
  titleKey: string
  items: PagePanelItem[]
}

const fallbackTitleKey = 'toc.onThisPage'

export function visiblePanelSections(
  sections: PagePanelSection[],
): PagePanelSection[] {
  return sections.filter((section) => section.items.length > 0)
}

// The panel is labelled after what it lists first: "On this page" only makes
// sense once there's something to jump to, a page with attachments but no
// headings is labelled after its attachments.
export function panelTitleKey(sections: PagePanelSection[]): string {
  const first = visiblePanelSections(sections)[0] ?? sections[0]
  return first?.titleKey ?? fallbackTitleKey
}
