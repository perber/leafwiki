import { describe, expect, it } from 'vitest'
import {
  panelTitleKey,
  visiblePanelSections,
  type PagePanelSection,
} from './pagePanelSection'

function section(
  id: string,
  titleKey: string,
  itemCount: number,
): PagePanelSection {
  return {
    id,
    titleKey,
    items: Array.from({ length: itemCount }, (_, i) => ({
      kind: 'anchor' as const,
      key: `${id}-${i}`,
      label: `${id} ${i}`,
      testId: `${id}-${i}`,
    })),
  }
}

// Headings, highlights, attachments — the order PageViewer composes them in.
function pageSections(
  headings: number,
  highlights: number,
  attachments: number,
) {
  return [
    section('headings', 'toc.onThisPage', headings),
    section('highlights', 'toc.highlights', highlights),
    section('attachments', 'toc.downloads', attachments),
  ]
}

describe('panelTitleKey', () => {
  it.each([
    { name: 'headings only', counts: [3, 0, 0], want: 'toc.onThisPage' },
    {
      name: 'headings and attachments',
      counts: [3, 0, 2],
      want: 'toc.onThisPage',
    },
    { name: 'all three', counts: [3, 1, 2], want: 'toc.onThisPage' },
    { name: 'highlights only', counts: [0, 1, 0], want: 'toc.highlights' },
    // The middle section titles the panel once it is the first non-empty one.
    {
      name: 'highlights and attachments',
      counts: [0, 1, 2],
      want: 'toc.highlights',
    },
    { name: 'attachments only', counts: [0, 0, 2], want: 'toc.downloads' },
    { name: 'none', counts: [0, 0, 0], want: 'toc.onThisPage' },
  ])(
    'titles the panel after the first non-empty section: $name',
    ({ counts: [headings, highlights, attachments], want }) => {
      expect(
        panelTitleKey(pageSections(headings, highlights, attachments)),
      ).toBe(want)
    },
  )

  it('falls back to "on this page" without any sections', () => {
    expect(panelTitleKey([])).toBe('toc.onThisPage')
  })
})

describe('visiblePanelSections', () => {
  it('drops empty sections and keeps the order of the rest', () => {
    const sections = [
      section('a', 'a', 1),
      section('b', 'b', 0),
      section('c', 'c', 2),
    ]
    expect(visiblePanelSections(sections).map((s) => s.id)).toEqual(['a', 'c'])
  })
})
