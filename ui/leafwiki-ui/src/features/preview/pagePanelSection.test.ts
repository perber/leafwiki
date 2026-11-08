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
    {
      name: 'headings only',
      headings: 3,
      attachments: 0,
      want: 'toc.onThisPage',
    },
    {
      name: 'headings and attachments',
      headings: 3,
      attachments: 2,
      want: 'toc.onThisPage',
    },
    {
      name: 'attachments only',
      headings: 0,
      attachments: 2,
      want: 'toc.downloads',
    },
    { name: 'neither', headings: 0, attachments: 0, want: 'toc.onThisPage' },
    {
      name: 'all three',
      headings: 3,
      highlights: 1,
      attachments: 2,
      want: 'toc.onThisPage',
    },
    {
      name: 'highlights only',
      headings: 0,
      highlights: 1,
      attachments: 0,
      want: 'toc.highlights',
    },
    {
      name: 'highlights and attachments',
      headings: 0,
      highlights: 1,
      attachments: 2,
      want: 'toc.highlights',
    },
  ])(
    'titles the panel after the first non-empty section: $name',
    ({ headings, highlights = 0, attachments, want }) => {
      expect(
        panelTitleKey(pageSections(headings, highlights, attachments)),
      ).toBe(want)
    },
  )

  it('uses a section placed between the others once it is the first non-empty one', () => {
    const sections = [
      section('headings', 'toc.onThisPage', 0),
      section('middle', 'toc.middle', 1),
      section('attachments', 'toc.downloads', 2),
    ]
    expect(panelTitleKey(sections)).toBe('toc.middle')
  })

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
