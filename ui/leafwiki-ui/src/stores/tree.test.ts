import { describe, it, expect, beforeEach } from 'vitest'
import { mergeDraftOverlay, useTreeStore } from './tree'
import type { PageNode } from '@/lib/api/pages'

const makeNode = (id: string, title: string, path: string): PageNode => ({
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

describe('mergeDraftOverlay', () => {
  it('appends pending drafts under a leaf without changing canonical input', () => {
    const root = {
      ...makeNode('root', 'Root', ''),
      slug: 'root',
      kind: 'section' as const,
      children: [makeNode('page-1', 'Published', 'published')],
    }
    const merged = mergeDraftOverlay(root, {
      drafts: [{ pageId: 'page-1' }],
      pending: [
        {
          id: 'pending-1',
          parentId: 'page-1',
          title: 'Pending',
          slug: 'pending',
        },
      ],
    })
    expect(root.children).toHaveLength(1)
    expect(merged.children?.[0].draft).toBe('active')
    expect(merged.children?.[0]).toMatchObject({ kind: 'page' })
    expect(merged.children?.[0].children?.[0]).toMatchObject({
      id: 'pending-1',
      draft: 'pending',
      parentId: 'page-1',
      path: 'published/pending',
    })
  })

  it('finds canonical ancestors for an overlay pending node', () => {
    const parent = makeNode('page-1', 'Published', 'published')
    const pending = {
      ...makeNode('pending-1', 'Pending', 'published/pending'),
      parentId: 'page-1',
      draft: 'pending' as const,
    }
    useTreeStore.setState({ byId: { 'page-1': parent, 'pending-1': pending } })
    expect(useTreeStore.getState().getAncestors('pending-1')).toEqual([
      'page-1',
    ])
  })

  it('keeps an orphaned pending draft reachable at the root but out of page lookup', () => {
    const root = {
      ...makeNode('root', 'Root', ''),
      slug: 'root',
      kind: 'section' as const,
      children: [],
    }
    const merged = mergeDraftOverlay(root, {
      drafts: [],
      pending: [
        {
          id: 'pending-1',
          parentId: 'missing-parent',
          title: 'Pending',
          slug: 'pending',
        },
      ],
    })
    expect(merged.children?.[0]).toMatchObject({
      id: 'pending-1',
      path: 'pending',
      draft: 'pending',
      parentId: 'root',
    })

    useTreeStore.setState({
      byId: { 'pending-1': merged.children?.[0] as PageNode },
    })
    expect(useTreeStore.getState().getPagesByTitle('Pending')).toEqual([])
  })

  it('keeps a published page title-resolvable when it has an active draft', () => {
    const active = {
      ...makeNode('page-1', 'Published', 'published'),
      draft: 'active' as const,
    }
    useTreeStore.setState({ byId: { 'page-1': active } })
    expect(useTreeStore.getState().getPagesByTitle('Published')).toEqual([
      active,
    ])
  })
})
