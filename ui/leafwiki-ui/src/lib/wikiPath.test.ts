import { describe, expect, it } from 'vitest'
import {
  getDeleteRedirectRoutePath,
  getParentWikiRoutePath,
  getWikiLinkSearchAndHash,
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
})

describe('getWikiLinkSearchAndHash', () => {
  it.each([
    { name: 'plain path', href: 'page-1', search: '', hash: '' },
    { name: 'hash', href: 'page-1#heading2', search: '', hash: '#heading2' },
    {
      name: 'absolute path with hash',
      href: '/docs/page-1#heading2',
      search: '',
      hash: '#heading2',
    },
    {
      name: 'query and hash',
      href: '../page-1?q=term#heading2',
      search: '?q=term',
      hash: '#heading2',
    },
    { name: 'empty hash', href: 'page-1#', search: '', hash: '' },
    {
      name: 'non-ASCII hash, encoded once',
      href: 'page-1#überschrift',
      search: '',
      hash: '#%C3%BCberschrift',
    },
    {
      name: 'already encoded hash, not encoded twice',
      href: 'page-1#%C3%BCberschrift',
      search: '',
      hash: '#%C3%BCberschrift',
    },
    {
      name: 'hash with spaces and quotes',
      href: 'page-1#a b"c',
      search: '',
      hash: '#a%20b%22c',
    },
    {
      name: 'markup-like hash, angle brackets encoded',
      href: 'page-1#"><img>',
      search: '',
      hash: '#%22%3E%3Cimg%3E',
    },
    {
      name: 'NUL escape kept as written',
      href: 'p#%00',
      search: '',
      hash: '#%00',
    },
    {
      name: 'second # kept in the hash',
      href: 'p#a#b',
      search: '',
      hash: '#a#b',
    },
    {
      name: '? after # kept in the hash',
      href: 'p#a?b',
      search: '',
      hash: '#a?b',
    },
    { name: 'empty query', href: 'p?#h', search: '', hash: '#h' },
    { name: 'hash only', href: '#h', search: '', hash: '#h' },
    { name: 'backslash path', href: '..\\p?x=1#h', search: '?x=1', hash: '#h' },
    {
      name: 'non-special scheme keeps its hash',
      href: 'javascript:alert(1)#h',
      search: '',
      hash: '#h',
    },
    {
      name: 'unparseable href, nothing',
      href: 'http://[#h',
      search: '',
      hash: '',
    },
  ])('returns the $name', ({ href, search, hash }) => {
    expect(getWikiLinkSearchAndHash(href)).toEqual({ search, hash })
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
