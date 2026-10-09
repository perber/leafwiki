import { describe, expect, it } from 'vitest'
import {
  buildFlatPageSearchItems,
  searchFlatPageSearchItems,
} from './pageSearch'
import type { PageNode } from './api/pages'
const node = (id: string, title: string, path: string): PageNode => ({
  id,
  title,
  path,
  slug: id,
  version: 'v1',
  kind: 'page',
  children: null,
})
const root: PageNode = {
  ...node('root', 'Root', ''),
  children: [
    node('1', 'Alpha Beta', 'alpha-beta'),
    node('2', 'Alpha', 'beta/alpha'),
    node('3', 'Gamma', 'alpha/beta'),
    node('4', 'Other', 'elsewhere'),
  ],
}
describe('searchFlatPageSearchItems', () => {
  it('preserves multiword fallback ranking and case/whitespace normalization', () => {
    const items = buildFlatPageSearchItems(root)
    expect(
      searchFlatPageSearchItems(items, '  ALPHA   beta  ').map((n) => n.id),
    ).toEqual(['2', '1', '3'])
    expect(
      searchFlatPageSearchItems(items, 'alpha beta', 1).map((n) => n.id),
    ).toEqual(['1'])
  })
  it('preserves title ranking, empty queries, and zero limits', () => {
    const items = buildFlatPageSearchItems(root)
    expect(searchFlatPageSearchItems(items, 'alpha').map((n) => n.id)).toEqual([
      '2',
      '1',
      '3',
    ])
    expect(searchFlatPageSearchItems(items, '', 2).map((n) => n.id)).toEqual([
      '1',
      '2',
    ])
    expect(searchFlatPageSearchItems(items, 'alpha', 0)).toEqual([])
    expect(searchFlatPageSearchItems(items, 'missing words')).toEqual([])
  })
})
