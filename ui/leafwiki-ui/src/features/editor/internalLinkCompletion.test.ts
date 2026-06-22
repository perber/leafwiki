import { CompletionContext } from '@codemirror/autocomplete'
import { EditorState } from '@codemirror/state'
import { describe, it, expect, beforeEach } from 'vitest'
import {
  hasSuppressedExternalPrefix,
  buildMarkdownLinkOptions,
  buildWikiLinkOptions,
  wikiLinkCompletionSource,
} from './internalLinkCompletion'
import type { FlatPageSearchItem } from '@/lib/pageSearch'
import { useTreeStore } from '@/stores/tree'

const item = (
  title: string,
  path: string,
  breadcrumb?: string,
): FlatPageSearchItem => ({
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

  it('suppresses mailto as a plain word without colon', () => {
    expect(hasSuppressedExternalPrefix('mailto')).toBe(true)
  })
})

describe('wikiLinkCompletionSource', () => {
  beforeEach(() => {
    useTreeStore.setState({ flatPages: [] })
  })

  it('replaces a single trailing closing bracket', () => {
    useTreeStore.setState({
      flatPages: [item('Target Page', 'docs/target-page')],
    })

    const doc = '[[Target]'
    const state = EditorState.create({ doc })
    const context = new CompletionContext(state, doc.length - 1, true)
    const result = wikiLinkCompletionSource(context)

    expect(result).not.toBeNull()
    expect(result?.from).toBe(2)
    expect(result?.to).toBe(doc.length)
    expect(result?.options[0]?.apply).toBe('Target Page]]')
  })

  it('does not replace text after the cursor on the same line', () => {
    useTreeStore.setState({
      flatPages: [item('Target Page', 'docs/target-page')],
    })

    const doc = 'Before [[Target after text'
    const cursorPos = 'Before [[Target'.length
    const state = EditorState.create({ doc })
    const context = new CompletionContext(state, cursorPos, true)
    const result = wikiLinkCompletionSource(context)

    expect(result).not.toBeNull()
    expect(result?.from).toBe('Before [['.length)
    expect(result?.to).toBe(cursorPos)

    const applied = `${doc.slice(0, result!.from)}${String(result!.options[0]!.apply)}${doc.slice(result!.to)}`
    expect(applied).toBe('Before [[Target Page]] after text')
  })
})

describe('buildMarkdownLinkOptions', () => {
  it('maps items to completion options with path-based apply', () => {
    const options = buildMarkdownLinkOptions([
      item('Intro', 'docs/intro', 'Docs / Intro'),
    ])
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
    const options = buildWikiLinkOptions([
      item('Intro', 'docs/intro', 'Docs / Intro'),
    ])
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
