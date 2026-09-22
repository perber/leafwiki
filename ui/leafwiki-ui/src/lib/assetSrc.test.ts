import { describe, expect, it } from 'vitest'
import { normalizeAssetSrc, versionAssetSrc } from './assetSrc'

describe('normalizeAssetSrc', () => {
  it('leaves an absolute external URL unchanged', () => {
    expect(normalizeAssetSrc('https://example.com/doc.pdf')).toBe(
      'https://example.com/doc.pdf',
    )
  })

  it('leaves a data URI unchanged', () => {
    const dataUri = 'data:image/png;base64,abc123'
    expect(normalizeAssetSrc(dataUri)).toBe(dataUri)
  })

  it('normalizes a /assets/-prefixed path', () => {
    expect(normalizeAssetSrc('/assets/foo.png')).toBe('/assets/foo.png')
  })

  it('adds a leading slash to an assets/-prefixed path (no leading slash)', () => {
    expect(normalizeAssetSrc('assets/foo.png')).toBe('/assets/foo.png')
  })

  it('normalizes a /api/-prefixed path', () => {
    expect(normalizeAssetSrc('/api/assets/foo.pdf')).toBe('/api/assets/foo.pdf')
  })
})

describe('versionAssetSrc', () => {
  it('appends a ?v= cache-busting param to a wiki asset src', () => {
    const versioned = versionAssetSrc('/assets/foo.png')
    expect(versioned).toContain('/assets/foo.png')
    expect(versioned).toMatch(/\?v=\d+$/)
  })

  it('does not double-append ?v= when the src already has one', () => {
    const versioned = versionAssetSrc('/assets/foo.png?v=123')
    expect(versioned).toMatch(/\/assets\/foo\.png\?v=123$/)
  })

  it('preserves a #fragment after the cache-busting param (PDF #page=N)', () => {
    const versioned = versionAssetSrc('/assets/manual.pdf#page=3')
    expect(versioned).toMatch(/\/assets\/manual\.pdf\?v=\d+#page=3$/)
  })

  it('normalizes a /api/-prefixed asset src and versions it too', () => {
    const versioned = versionAssetSrc('/api/assets/foo.pdf')
    expect(versioned).toMatch(/\/api\/assets\/foo\.pdf\?v=\d+$/)
  })

  it('leaves an external URL normalized but unversioned', () => {
    expect(versionAssetSrc('https://example.com/doc.pdf')).toBe(
      'https://example.com/doc.pdf',
    )
  })
})
