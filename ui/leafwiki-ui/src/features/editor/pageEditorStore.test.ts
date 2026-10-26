import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getPageByPath, updatePage } from '@/lib/api/pages'
import type { Page } from '@/lib/api/pages'
import { useTreeStore } from '@/stores/tree'
import { useLinkStatusStore } from '../links/linkStatusStore'
import { isDirtyState, usePageEditorStore } from './pageEditorStore'

vi.mock('@/lib/api/pages', async () => {
  const actual =
    await vi.importActual<typeof import('@/lib/api/pages')>('@/lib/api/pages')
  return {
    ...actual,
    getPageByPath: vi.fn(),
    updatePage: vi.fn(),
  }
})

const fakePage: Page = {
  id: 'page-1',
  title: 'Getting Started',
  slug: 'getting-started',
  path: 'docs/getting-started',
  kind: 'page',
  content: 'Hello world',
  version: 'v1',
  tags: ['guide'],
  properties: { owner: 'alice' },
} as Page

describe('pageEditorStore.resetEditorState', () => {
  beforeEach(() => {
    usePageEditorStore.setState(usePageEditorStore.getInitialState())
  })

  it('clears page back to null so stale currentEditorPageId reads disappear', () => {
    usePageEditorStore.setState({
      page: fakePage,
      initialPage: fakePage,
      title: fakePage.title,
      slug: fakePage.slug,
      content: fakePage.content,
      tags: fakePage.tags,
      frontmatterFields: [{ key: 'owner', value: 'alice', type: 'text' }],
      notFound: true,
      error: 'stale error',
    })

    usePageEditorStore.getState().resetEditorState()

    const state = usePageEditorStore.getState()
    expect(state.page).toBeNull()
    expect(state.initialPage).toBeNull()
    expect(state.title).toBe('')
    expect(state.slug).toBe('')
    expect(state.content).toBe('')
    expect(state.tags).toEqual([])
    expect(state.frontmatterFields).toEqual([])
    expect(state.notFound).toBe(false)
    expect(state.error).toBeNull()
  })

  it('leaves the store in a clean state when nothing was ever loaded', () => {
    usePageEditorStore.getState().resetEditorState()

    expect(usePageEditorStore.getState().page).toBeNull()
  })
})

describe('pageEditorStore.savePage', () => {
  const pageA: Page = {
    ...fakePage,
    id: 'page-a',
    title: 'A',
    slug: 'a',
    path: 'a',
    content: 'A content',
    version: 'a1',
    tags: ['a-tag'],
    properties: { owner: 'alice' },
  }
  const pageB: Page = {
    ...fakePage,
    id: 'page-b',
    title: 'B',
    slug: 'b',
    path: 'b',
    content: 'B content',
    version: 'b1',
    tags: ['b-tag'],
    properties: { owner: 'bob' },
  }

  function editPage(page: Page, content: string) {
    usePageEditorStore.setState({
      page: { ...page },
      initialPage: { ...page },
      title: page.title,
      slug: page.slug,
      content,
      tags: page.tags,
      frontmatterFields: [
        { key: 'owner', value: String(page.properties?.owner), type: 'text' },
      ],
    })
  }

  function deferUpdatePage() {
    let resolve: (page: Page) => void = () => {}
    vi.mocked(updatePage).mockReturnValueOnce(
      new Promise<Page>((r) => {
        resolve = r
      }),
    )
    return (page: Page) => resolve(page)
  }

  beforeEach(() => {
    vi.clearAllMocks()
    usePageEditorStore.setState(usePageEditorStore.getInitialState())
    useTreeStore.setState({ patchNodeVersion: vi.fn() })
    useLinkStatusStore.setState({ fetchLinkStatusForPage: vi.fn() })
  })

  it('applies the response to the page it saved and keeps edits typed meanwhile', async () => {
    editPage(pageA, 'A edited')
    const resolveSave = deferUpdatePage()

    const before = usePageEditorStore.getState().page
    const saving = usePageEditorStore.getState().savePage({ silent: true })
    usePageEditorStore.getState().setContent('A edited further')
    resolveSave({ ...pageA, content: 'A edited', version: 'a2' })
    await saving

    const state = usePageEditorStore.getState()
    expect(state.page?.content).toBe('A edited')
    expect(state.page?.version).toBe('a2')
    expect(state.content).toBe('A edited further')
    expect(before?.version).toBe('a1')
  })

  it('leaves a page opened while the save was in flight untouched', async () => {
    editPage(pageA, 'A edited')
    const resolveSave = deferUpdatePage()

    const saving = usePageEditorStore.getState().savePage({ silent: true })
    vi.mocked(getPageByPath).mockResolvedValueOnce({ ...pageB })
    await usePageEditorStore.getState().loadPageData('b')
    resolveSave({ ...pageA, content: 'A edited', version: 'a2' })
    await saving

    const state = usePageEditorStore.getState()
    expect(state.page).toEqual(pageB)
    expect(state.tags).toEqual(['b-tag'])
    expect(state.frontmatterFields).toEqual([
      { key: 'owner', value: 'bob', type: 'text' },
    ])
    expect(isDirtyState(state)).toBe(false)
  })

  it('leaves a reopened page untouched when its earlier save completes late', async () => {
    editPage(pageA, 'A edited')
    const resolveSave = deferUpdatePage()

    const saving = usePageEditorStore.getState().savePage({ silent: true })
    vi.mocked(getPageByPath).mockResolvedValueOnce({ ...pageA })
    await usePageEditorStore.getState().loadPageData('a')
    resolveSave({ ...pageA, content: 'A edited', version: 'a2' })
    await saving

    expect(usePageEditorStore.getState().page).toEqual(pageA)
  })
})
