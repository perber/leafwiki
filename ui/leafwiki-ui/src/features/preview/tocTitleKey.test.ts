import { describe, expect, it } from 'vitest'
import { tocTitleKey } from './tocTitleKey'

describe('tocTitleKey', () => {
  it('prefers "on this page" when there are headings', () => {
    expect(tocTitleKey(3, 0)).toBe('toc.onThisPage')
    expect(tocTitleKey(3, 2)).toBe('toc.onThisPage')
  })

  it('falls back to "downloads" when there are only attachments', () => {
    expect(tocTitleKey(0, 2)).toBe('toc.downloads')
  })

  it('falls back to "on this page" when neither headings nor attachments exist', () => {
    expect(tocTitleKey(0, 0)).toBe('toc.onThisPage')
  })
})
