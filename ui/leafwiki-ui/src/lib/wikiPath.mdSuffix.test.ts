import { describe, expect, it } from 'vitest'
import { resolveWikiLinkPath } from './wikiPath'

describe('resolveWikiLinkPath markdown suffix', () => {
  it('strips a trailing .md from relative targets', () => {
    expect(resolveWikiLinkPath('/docs/guide', 'setup.md')).toBe('/docs/guide/setup')
    expect(resolveWikiLinkPath('/docs/guide', '../other.md')).toBe('/docs/other')
  })

  it('strips .md case-insensitively', () => {
    expect(resolveWikiLinkPath('/docs/guide', 'Setup.MD')).toBe('/docs/guide/Setup')
  })
})