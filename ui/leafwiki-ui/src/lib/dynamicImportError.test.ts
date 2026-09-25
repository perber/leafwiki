import { describe, expect, it } from 'vitest'
import { isDynamicImportChunkError } from './dynamicImportError'

describe('isDynamicImportChunkError', () => {
  it('matches the Chromium/V8 message (reproduced from a live redeploy)', () => {
    expect(
      isDynamicImportChunkError(
        'Failed to fetch dynamically imported module: https://stuart.cloud.leafwiki.com/static/flowDiagram-HODETNUW-DScmEErP.js',
      ),
    ).toBe(true)
  })

  it('matches the Firefox message', () => {
    expect(
      isDynamicImportChunkError(
        'error loading dynamically imported module: https://example.com/static/flowDiagram-abc.js',
      ),
    ).toBe(true)
  })

  it('matches the Safari/WebKit message', () => {
    expect(isDynamicImportChunkError('Importing a module script failed.')).toBe(
      true,
    )
  })

  it('does not match an unrelated Mermaid parse error', () => {
    expect(
      isDynamicImportChunkError('Parse error on line 1: Expecting...'),
    ).toBe(false)
  })

  it('does not match a generic empty or unrelated message', () => {
    expect(isDynamicImportChunkError('')).toBe(false)
    expect(isDynamicImportChunkError('No SVG element in Mermaid output')).toBe(
      false,
    )
  })
})
