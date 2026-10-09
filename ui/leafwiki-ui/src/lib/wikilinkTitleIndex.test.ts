import { describe, it, expect } from 'vitest'
import { buildWikiTitleIndex, preprocessWikilinks } from './preprocessWikilinks'
import type { PageNode } from './api/pages'
const page = (id: string, title: string): PageNode => ({
  id,
  title,
  path: id,
  slug: id,
  version: 'v1',
  kind: 'page',
  children: null,
})
describe('buildWikiTitleIndex', () => {
  it('preserves Unicode case folding, duplicate order and special map keys', () => {
    const first = page('1', 'Überblick'),
      second = page('2', 'ÜBERBLICK'),
      special = page('3', '__proto__')
    const index = buildWikiTitleIndex({ first, second, special })
    expect(index.get('überblick')).toEqual([first, second])
    expect(index.get('__proto__')).toEqual([special])
    expect(index.get('missing')).toBeUndefined()
    expect(
      preprocessWikilinks(
        '[[Überblick]] [[__proto__]]',
        (title) => index.get(title.toLowerCase()) ?? [],
      ),
    ).toBe('[Überblick](wikilink-ambiguous:%C3%9Cberblick) [__proto__](/3)')
  })
  it('rebuilds after a title changes without mutating the old index', () => {
    const old = page('1', 'Before')
    const first = buildWikiTitleIndex({ old })
    const next = buildWikiTitleIndex({ old: { ...old, title: 'After' } })
    expect(first.get('before')).toEqual([old])
    expect(next.has('before')).toBe(false)
    expect(next.get('after')?.[0].id).toBe('1')
  })
})
