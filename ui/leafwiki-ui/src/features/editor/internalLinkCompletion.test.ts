import { describe, it, expect } from 'vitest'
import {
  hasSuppressedExternalPrefix,
  buildMarkdownLinkOptions,
  buildWikiLinkOptions,
} from './internalLinkCompletion'
import type { FlatPageSearchItem } from '@/lib/pageSearch'

const item = (title: string, path: string, breadcrumb?: string): FlatPageSearchItem => ({
  id: path,
  title,
  path,
  kind: 'page',
  breadcrumb: breadcrumb ?? title,
  searchText: `${title} ${path}`,
  normalizedTitle: title.toLowerCase().trim(),
  normalizedPath: path.toLowerCase().trim(),
  normalizedBreadcrumb: (breadcrumb ?? title).toLowerCase().trim(),
})

describe('hasSuppressedExternalPrefix', () => {
  it('suppresses http', () => {
    expect(hasSuppressedExternalPrefix('http')).toBe(true)
  })

  it('suppresses http: and https:', () => {
    expect(hasSuppressedExternalPrefix('http:')).toBe(true)
    expect(hasSuppressedExternalPrefix('https:')).toBe(true)
  })

  it('suppresses full external URLs', () => {
    expect(hasSuppressedExternalPrefix('https://example.com')).toBe(true)
    expect(hasSuppressedExternalPrefix('mailto:user@example.com')).toBe(true)
  })

  it('suppresses with leading whitespace', () => {
    expect(hasSuppressedExternalPrefix('  https://example.com')).toBe(true)
  })

  it('is case-insensitive', () => {
    expect(hasSuppressedExternalPrefix('HTTP://example.com')).toBe(true)
    expect(hasSuppressedExternalPrefix('MAILTO:user@example.com')).toBe(true)
  })

  it('does not suppress internal paths', () => {
    expect(hasSuppressedExternalPrefix('/docs/intro')).toBe(false)
    expect(hasSuppressedExternalPrefix('docs/intro')).toBe(false)
    expect(hasSuppressedExternalPrefix('')).toBe(false)
  })

  it('does not suppress mailto as a plain word without colon', () => {
    expect(hasSuppressedExternalPrefix('mailto')).toBe(true)
  })
})

describe('buildMarkdownLinkOptions', () => {
  it('maps items to completion options with path-based apply', () => {
    const options = buildMarkdownLinkOptions([item('Intro', 'docs/intro', 'Docs / Intro')])
    expect(options).toHaveLength(1)
    expect(options[0].label).toBe('Intro')
    expect(options[0].displayLabel).toBe('Intro')
    expect(options[0].apply).toBe('/docs/intro')
    expect(options[0].info).toBe('Docs / Intro')
    expect(options[0].path).toBe('docs/intro')
  })

  it('returns one option per item', () => {
    const options = buildMarkdownLinkOptions([
      item('Page A', 'a'),
      item('Page B', 'b'),
      item('Page C', 'c'),
    ])
    expect(options).toHaveLength(3)
    expect(options.map((o) => o.apply)).toEqual(['/a', '/b', '/c'])
  })
})

describe('buildWikiLinkOptions', () => {
  it('maps items to completion options with title-based apply closing ]]', () => {
    const options = buildWikiLinkOptions([item('Intro', 'docs/intro', 'Docs / Intro')])
    expect(options).toHaveLength(1)
    expect(options[0].label).toBe('Intro')
    expect(options[0].displayLabel).toBe('Intro')
    expect(options[0].apply).toBe('Intro]]')
    expect(options[0].info).toBe('Docs / Intro')
    expect(options[0].path).toBe('docs/intro')
  })

  it('uses the page title (not path) in apply', () => {
    const options = buildWikiLinkOptions([item('My Page', 'folder/my-page')])
    expect(options[0].apply).toBe('My Page]]')
  })

  it('returns one option per item', () => {
    const options = buildWikiLinkOptions([
      item('Page A', 'a'),
      item('Page B', 'b'),
    ])
    expect(options).toHaveLength(2)
    expect(options.map((o) => o.apply)).toEqual(['Page A]]', 'Page B]]'])
  })
})
