import { describe, it, expect, beforeEach } from 'vitest'
import { useTreeStore } from './tree'
import type { PageNode } from '@/lib/api/pages'

const makeNode = (
  id: string,
  title: string,
  path: string,
): PageNode => ({
  id,
  title,
  slug: path.split('/').pop() ?? path,
  path,
  version: '1',
  kind: 'page',
  children: null,
})

describe('useTreeStore — getPagesByTitle', () => {
  beforeEach(() => {
    useTreeStore.setState({
      byId: {},
      byPath: {},
      tree: null,
      flatPages: [],
    })
  })

  const load = (nodes: PageNode[]) => {
    const byId: Record<string, PageNode> = {}
    const byPath: Record<string, PageNode> = {}
    for (const n of nodes) {
      byId[n.id] = n
      byPath[n.path] = n
    }
    useTreeStore.setState({ byId, byPath })
  }

  it('returns a matching page case-insensitively', () => {
    load([makeNode('1', 'Getting Started', 'docs/getting-started')])
    const { getPagesByTitle } = useTreeStore.getState()
    expect(getPagesByTitle('Getting Started')).toHaveLength(1)
    expect(getPagesByTitle('getting started')).toHaveLength(1)
    expect(getPagesByTitle('GETTING STARTED')).toHaveLength(1)
  })

  it('returns all pages that share a title', () => {
    load([
      makeNode('1', 'Notes', 'team/notes'),
      makeNode('2', 'Notes', 'personal/notes'),
    ])
    const { getPagesByTitle } = useTreeStore.getState()
    expect(getPagesByTitle('Notes')).toHaveLength(2)
  })

  it('returns an empty array when no page matches', () => {
    load([makeNode('1', 'Intro', 'intro')])
    const { getPagesByTitle } = useTreeStore.getState()
    expect(getPagesByTitle('Nonexistent')).toHaveLength(0)
  })

  it('returns an empty array when the tree is empty', () => {
    const { getPagesByTitle } = useTreeStore.getState()
    expect(getPagesByTitle('Anything')).toHaveLength(0)
  })
})
