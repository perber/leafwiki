import { describe, expect, it } from 'vitest'
import { buildAttachmentsSection } from './attachmentsSection'

describe('buildAttachmentsSection', () => {
  it('maps each attachment to a download item in the toc.downloads section', () => {
    const section = buildAttachmentsSection([
      { name: 'notes.pdf', url: '/assets/p1/notes.pdf' },
    ])

    expect(section).toEqual({
      id: 'attachments',
      titleKey: 'toc.downloads',
      items: [
        {
          kind: 'download',
          key: '/assets/p1/notes.pdf',
          label: 'notes.pdf',
          href: '/assets/p1/notes.pdf',
          fileName: 'notes.pdf',
          testId: 'download-notes.pdf',
        },
      ],
    })
  })

  it('returns an empty section without attachments', () => {
    expect(buildAttachmentsSection([]).items).toEqual([])
  })
})
