import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Page } from '@/lib/api/pages'

const api = vi.hoisted(() => ({
  discardDraft: vi.fn(),
  discardPendingDraft: vi.fn(),
  getDraft: vi.fn(),
  getPageByPath: vi.fn(),
  getPendingDraft: vi.fn(),
  publishDraft: vi.fn(),
  publishPendingDraft: vi.fn(),
  saveDraft: vi.fn(),
  savePendingDraft: vi.fn(),
  startDraft: vi.fn(),
}))

vi.mock('@/lib/api/pages', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/api/pages')>()),
  ...api,
}))

vi.mock('../links/linkstatus_store', () => ({
  useLinkStatusStore: {
    getState: () => ({ fetchLinkStatusForPage: vi.fn() }),
  },
}))

import { usePageEditorStore } from './pageEditorStore'

const published: Page = {
  id: 'page-1',
  title: 'Published',
  slug: 'published',
  path: 'published',
  kind: 'page',
  content: 'public content',
  version: 'v1',
  tags: [],
  properties: {},
} as Page

const draftPage: Page = {
  ...published,
  content: 'stored draft',
}

const secondPage: Page = {
  ...published,
  id: 'page-2',
  title: 'Second',
  slug: 'second',
  path: 'second',
  content: 'second public content',
}

function loadEditor(page: Page, isDraft: boolean) {
  usePageEditorStore.setState({
    page,
    initialPage: { ...page },
    title: page.title,
    slug: page.slug,
    content: page.content,
    tags: page.tags ?? [],
    frontmatterFields: [],
    isDraft,
  })
}

describe('pageEditorStore drafts', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    usePageEditorStore.setState(usePageEditorStore.getInitialState())
  })

  it('switches an existing published page to its stored working draft', async () => {
    api.startDraft.mockResolvedValue({ page: draftPage, baseVersion: 'v1' })
    loadEditor(published, false)

    await usePageEditorStore.getState().startDraft()

    expect(api.startDraft).toHaveBeenCalledWith('page-1')
    expect(usePageEditorStore.getState()).toMatchObject({
      isDraft: true,
      content: 'stored draft',
    })
  })

  it('persists dirty draft edits before publishing', async () => {
    const savedDraft = { ...draftPage, content: 'latest edit' }
    const publishedResult = {
      ...published,
      content: 'latest edit',
      version: 'v2',
    }
    api.saveDraft.mockResolvedValue({ page: savedDraft, baseVersion: 'v1' })
    api.publishDraft.mockResolvedValue(publishedResult)
    loadEditor(draftPage, true)
    usePageEditorStore.setState({ content: 'latest edit' })

    await usePageEditorStore.getState().publishDraft()

    expect(api.saveDraft).toHaveBeenCalledWith(
      'page-1',
      'Published',
      'latest edit',
      [],
      {},
    )
    expect(api.saveDraft.mock.invocationCallOrder[0]).toBeLessThan(
      api.publishDraft.mock.invocationCallOrder[0],
    )
    expect(usePageEditorStore.getState()).toMatchObject({
      isDraft: false,
      content: 'latest edit',
    })
  })

  it('returns to the canonical page after discarding', async () => {
    api.discardDraft.mockResolvedValue(undefined)
    api.getPageByPath.mockResolvedValue(published)
    loadEditor(draftPage, true)

    await usePageEditorStore.getState().discardDraft()

    expect(api.discardDraft).toHaveBeenCalledWith('page-1')
    expect(usePageEditorStore.getState()).toMatchObject({
      isDraft: false,
      content: 'public content',
    })
  })

  it('keeps the latest page when an aborted draft load resolves late', async () => {
    let resolveFirstDraft!: (value: { page: Page; baseVersion: string }) => void
    const firstDraft = new Promise<{ page: Page; baseVersion: string }>(
      (resolve) => {
        resolveFirstDraft = resolve
      },
    )
    api.getPageByPath.mockImplementation((path: string) =>
      Promise.resolve(path === 'first' ? published : secondPage),
    )
    api.getDraft.mockImplementation((id: string) =>
      id === published.id
        ? firstDraft
        : Promise.resolve({
            page: { ...secondPage, content: 'second draft' },
            baseVersion: 'v1',
          }),
    )

    const first = usePageEditorStore.getState().loadPageData('first')
    await vi.waitFor(() =>
      expect(api.getDraft).toHaveBeenCalledWith(
        published.id,
        expect.any(AbortSignal),
      ),
    )
    const firstSignal = api.getDraft.mock.calls[0][1] as AbortSignal
    const second = usePageEditorStore.getState().loadPageData('second')
    await second
    expect(firstSignal.aborted).toBe(true)

    resolveFirstDraft({
      page: { ...published, content: 'first draft' },
      baseVersion: 'v1',
    })
    await first

    expect(usePageEditorStore.getState()).toMatchObject({
      page: expect.objectContaining({ id: secondPage.id }),
      content: 'second draft',
      isDraft: true,
    })
  })

  it('reloads, saves, publishes, and discards a pending draft', async () => {
    const pending = {
      ...draftPage,
      id: 'pending-1',
      path: 'pending',
      version: 'pending',
    }
    api.getPendingDraft.mockResolvedValue({
      page: pending,
      pending: true,
      parentId: '',
    })
    api.savePendingDraft.mockResolvedValue({
      page: { ...pending, content: 'saved' },
      pending: true,
      parentId: '',
    })
    api.publishPendingDraft.mockResolvedValue({
      ...pending,
      id: 'published-1',
      path: 'published',
      version: 'v1',
      content: 'saved',
    })

    await usePageEditorStore.getState().loadPendingDraft('pending-1')
    usePageEditorStore.getState().setContent('saved')
    await usePageEditorStore.getState().savePage()
    expect(api.savePendingDraft).toHaveBeenCalledWith(
      'pending-1',
      'Published',
      'published',
      'saved',
      [],
      {},
    )
    await usePageEditorStore.getState().publishDraft()
    expect(api.publishPendingDraft).toHaveBeenCalledWith('pending-1')
    expect(usePageEditorStore.getState()).toMatchObject({
      isPendingDraft: false,
      page: expect.objectContaining({ id: 'published-1' }),
    })

    api.getPendingDraft.mockResolvedValue({
      page: pending,
      pending: true,
      parentId: '',
    })
    await usePageEditorStore.getState().loadPendingDraft('pending-1')
    await usePageEditorStore.getState().discardDraft()
    expect(api.discardPendingDraft).toHaveBeenCalledWith('pending-1')
    expect(usePageEditorStore.getState().page).toBeNull()
  })
})
