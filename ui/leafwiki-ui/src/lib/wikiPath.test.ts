import { describe, expect, it } from 'vitest'
import {
  getDeleteRedirectRoutePath,
  getParentWikiRoutePath,
  getWikiTargetRoutePath,
  normalizeWikiRoutePath,
  resolveWikiLinkPath,
  toWikiLookupPath,
} from './wikiPath'

describe('normalizeWikiRoutePath', () => {
  it('adds a leading slash and strips trailing slashes', () => {
    expect(normalizeWikiRoutePath('docs/guide')).toBe('/docs/guide')
    expect(normalizeWikiRoutePath('/docs/guide/')).toBe('/docs/guide')
  })

  it('strips query and hash fragments', () => {
    expect(normalizeWikiRoutePath('/docs/guide?q=1#section')).toBe(
      '/docs/guide',
    )
  })

  it('keeps root as /', () => {
    expect(normalizeWikiRoutePath('/')).toBe('/')
  })
})

describe('toWikiLookupPath', () => {
  it('converts absolute wiki routes to tree lookup keys', () => {
    expect(toWikiLookupPath('/docs/getting-started')).toBe(
      'docs/getting-started',
    )
    expect(toWikiLookupPath('/')).toBe('')
  })
})

describe('getWikiTargetRoutePath', () => {
  it('maps edit and history routes back to the view path', () => {
    expect(getWikiTargetRoutePath('/e/docs/guide')).toBe('/docs/guide')
    expect(getWikiTargetRoutePath('/history/docs/guide')).toBe('/docs/guide')
    expect(getWikiTargetRoutePath('/history')).toBe('/')
    expect(getWikiTargetRoutePath('/docs/guide')).toBe('/docs/guide')
  })
})

describe('resolveWikiLinkPath', () => {
  it('resolves relative links with page-as-folder semantics', () => {
    expect(resolveWikiLinkPath('/docs/guide', 'setup')).toBe(
      '/docs/guide/setup',
    )
    expect(resolveWikiLinkPath('/docs/guide', '../other')).toBe('/docs/other')
    expect(resolveWikiLinkPath('/docs/guide', './child/page')).toBe(
      '/docs/guide/child/page',
    )
  })

  it('resolves absolute wiki paths from the site root', () => {
    expect(resolveWikiLinkPath('/docs/guide', '/root-page')).toBe('/root-page')
  })

  it('resolves relative links with filesystem semantics when requested', () => {
    expect(resolveWikiLinkPath('/docs/guide', 'sibling', 'filesystem')).toBe(
      '/docs/sibling',
    )
    expect(resolveWikiLinkPath('/docs/guide', '../other', 'filesystem')).toBe(
      '/other',
    )
    expect(resolveWikiLinkPath('/docs/guide', './child', 'filesystem')).toBe(
      '/docs/child',
    )
    expect(
      resolveWikiLinkPath('/docs/nested/guide', '../other', 'filesystem'),
    ).toBe('/docs/other')
    // absolute wiki paths are unaffected by the mode
    expect(resolveWikiLinkPath('/docs/guide', '/root-page', 'filesystem')).toBe(
      '/root-page',
    )
  })

  it('defaults to page-folder semantics when no mode is passed', () => {
    expect(resolveWikiLinkPath('/docs/guide', 'setup')).toBe(
      '/docs/guide/setup',
    )
  })

  it('treats section (index) pages as their own directory in filesystem mode', () => {
    // wiki/foo/index.md lives IN wiki/foo/: links from a section page
    // resolve against the section itself, not its parent
    expect(
      resolveWikiLinkPath('/docs/tools', 'usage.md', 'filesystem', true),
    ).toBe('/docs/tools/usage')
    expect(
      resolveWikiLinkPath(
        '/docs/decisions',
        '../policies/index.md',
        'filesystem',
        true,
      ),
    ).toBe('/docs/policies')
    // leaf pages still resolve against the parent
    expect(
      resolveWikiLinkPath('/docs/guide', 'sibling', 'filesystem', false),
    ).toBe('/docs/sibling')
  })

  it('canonicalizes ".md" tails and trailing index segments to the section route', () => {
    expect(resolveWikiLinkPath('/docs/guide', 'sibling.md', 'filesystem')).toBe(
      '/docs/sibling',
    )
    expect(
      resolveWikiLinkPath('/docs/guide', 'accounts/index.md', 'filesystem'),
    ).toBe('/docs/accounts')
    expect(resolveWikiLinkPath('/docs/guide', 'index.md', 'filesystem')).toBe(
      '/docs',
    )
    expect(
      resolveWikiLinkPath('/docs/guide', 'setup/index', 'page-folder'),
    ).toBe('/docs/guide/setup')
  })
})

describe('getParentWikiRoutePath', () => {
  it('returns / for top-level pages', () => {
    expect(getParentWikiRoutePath('/docs')).toBe('/')
    expect(getParentWikiRoutePath('/')).toBe('/')
  })

  it('returns the parent for nested pages', () => {
    expect(getParentWikiRoutePath('/docs/guide/setup')).toBe('/docs/guide')
  })
})

describe('getDeleteRedirectRoutePath', () => {
  it('redirects to the parent when the deleted page is open', () => {
    expect(getDeleteRedirectRoutePath('/docs/guide', '/docs/guide')).toBe(
      '/docs',
    )
  })

  it('redirects from editor and history routes for the deleted page', () => {
    expect(getDeleteRedirectRoutePath('/e/docs/guide', '/docs/guide')).toBe(
      '/docs',
    )
    expect(
      getDeleteRedirectRoutePath('/history/docs/guide', '/docs/guide'),
    ).toBe('/docs')
  })

  it('redirects to the parent when a nested route under the deleted page is open', () => {
    expect(getDeleteRedirectRoutePath('/docs/guide/setup', '/docs/guide')).toBe(
      '/docs',
    )
  })

  it('keeps the current route when another page is open', () => {
    expect(getDeleteRedirectRoutePath('/docs/other', '/docs/guide')).toBe(
      '/docs/other',
    )
  })
})
