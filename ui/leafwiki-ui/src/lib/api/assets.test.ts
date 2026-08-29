import { describe, expect, it } from 'vitest'
import { listNonImageAttachments } from './assets'

describe('listNonImageAttachments', () => {
  it('keeps non-image assets and normalizes URLs', () => {
    expect(
      listNonImageAttachments([
        '/assets/p1/notes.pdf',
        'assets/p1/guide.docx',
      ]),
    ).toEqual([
      { name: 'notes.pdf', url: '/assets/p1/notes.pdf' },
      { name: 'guide.docx', url: '/assets/p1/guide.docx' },
    ])
  })

  it('filters out image extensions and deduplicates URLs', () => {
    expect(
      listNonImageAttachments([
        '/assets/p1/photo.png',
        '/assets/p1/notes.pdf',
        'assets/p1/notes.pdf',
      ]),
    ).toEqual([{ name: 'notes.pdf', url: '/assets/p1/notes.pdf' }])
  })
})
