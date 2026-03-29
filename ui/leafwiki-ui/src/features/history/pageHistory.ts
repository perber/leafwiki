import { mapApiError, type ApiUiError } from '@/lib/api/errors'
import {
  compareRevisions,
  getLatestRevision,
  getRevisionSnapshot,
  listRevisions,
  type Revision,
  type RevisionComparison,
  type RevisionSnapshot,
} from '@/lib/api/revisions'
import { useEffect } from 'react'
import { create } from 'zustand'
import { useProgressbarStore } from '../progressbar/progressbar'

export type HistoryTab = 'changes' | 'preview' | 'raw' | 'assets'

type PageHistoryState = {
  pageId: string
  revisions: Revision[]
  selectedRevisionId: string | null
  latestRevisionId: string | null
  isRevisionViewOpen: boolean
  snapshot: RevisionSnapshot | null
  comparison: RevisionComparison | null
  activeTab: HistoryTab
  listLoading: boolean
  previewLoading: boolean
  compareLoading: boolean
  listError: ApiUiError | null
  previewError: ApiUiError | null
  nextCursor: string
  loadingMore: boolean
}

type PageHistoryStore = PageHistoryState & {
  update: (patch: Partial<PageHistoryState>) => void
  reset: () => void
  selectRevision: (revisionId: string) => void
  openRevisionView: () => void
  closeRevisionView: () => void
  setActiveTab: (tab: HistoryTab) => void
}

const initialState: PageHistoryState = {
  pageId: '',
  revisions: [],
  selectedRevisionId: null,
  latestRevisionId: null,
  isRevisionViewOpen: false,
  snapshot: null,
  comparison: null,
  activeTab: 'changes',
  listLoading: false,
  previewLoading: false,
  compareLoading: false,
  listError: null,
  previewError: null,
  nextCursor: '',
  loadingMore: false,
}

async function loadPageHistoryState(pageId: string, update: (patch: Partial<PageHistoryState>) => void) {
  update({
    pageId,
    revisions: [],
    selectedRevisionId: null,
    latestRevisionId: null,
    isRevisionViewOpen: false,
    snapshot: null,
    comparison: null,
    activeTab: 'changes',
    listLoading: true,
    previewLoading: false,
    compareLoading: false,
    listError: null,
    previewError: null,
    nextCursor: '',
    loadingMore: false,
  })

  try {
    const historyData = await listRevisions(pageId)

    if (historyData.revisions.length === 0) {
      update({
        revisions: [],
        nextCursor: historyData.nextCursor,
        latestRevisionId: null,
        selectedRevisionId: null,
      })
      return
    }

    const latestRevision = await getLatestRevision(pageId)

    const visibleRevisions = excludeLatestRevision(
      historyData.revisions,
      latestRevision.id,
    )
    const firstVisibleRevision = visibleRevisions[0] ?? null

    update({
      revisions: visibleRevisions,
      nextCursor: historyData.nextCursor,
      latestRevisionId: latestRevision.id,
      selectedRevisionId: firstVisibleRevision?.id ?? null,
    })
  } catch (err) {
    update({
      listError: mapApiError(err, 'Failed to load page history'),
      revisions: [],
      nextCursor: '',
      latestRevisionId: null,
    })
  } finally {
    update({ listLoading: false })
  }
}

export const usePageHistoryStore = create<PageHistoryStore>((set) => ({
  ...initialState,
  update: (patch) => set((state) => ({ ...state, ...patch })),
  reset: () => set(initialState),
  selectRevision: (revisionId) =>
    set({
      selectedRevisionId: revisionId,
      isRevisionViewOpen: true,
      previewError: null,
      snapshot: null,
      comparison: null,
    }),
  openRevisionView: () => set({ isRevisionViewOpen: true }),
  closeRevisionView: () =>
    set({
      isRevisionViewOpen: false,
      previewError: null,
      snapshot: null,
      comparison: null,
    }),
  setActiveTab: (activeTab) => set({ activeTab }),
}))

function excludeLatestRevision(
  revisions: Revision[],
  latestRevisionId: string | null,
): Revision[] {
  if (!latestRevisionId) return revisions

  return revisions.filter((revision) => revision.id !== latestRevisionId)
}

export function usePageHistory(pageId: string | null, enabled = true) {
  const update = usePageHistoryStore((state) => state.update)
  const reset = usePageHistoryStore((state) => state.reset)
  const selectedRevisionId = usePageHistoryStore(
    (state) => state.selectedRevisionId,
  )
  const latestRevisionId = usePageHistoryStore(
    (state) => state.latestRevisionId,
  )
  const activeTab = usePageHistoryStore((state) => state.activeTab)

  useEffect(() => {
    if (!pageId) {
      reset()
      return
    }

    if (!enabled) {
      return
    }

    let cancelled = false

    const load = async () => {
      await loadPageHistoryState(pageId, (patch) => {
        if (!cancelled) {
          update(patch)
        }
      })
    }

    void load()

    return () => {
      cancelled = true
    }
  }, [enabled, pageId, reset, update])

  useEffect(() => {
    if (
      !pageId ||
      !selectedRevisionId ||
      (activeTab !== 'preview' && activeTab !== 'raw')
    ) {
      return
    }

    let cancelled = false

    const loadSnapshot = async () => {
      useProgressbarStore.getState().setLoading(true)
      update({
        previewLoading: true,
        previewError: null,
      })
      try {
        const data = await getRevisionSnapshot(pageId, selectedRevisionId)
        if (cancelled) return
        update({ snapshot: data })
      } catch (err) {
        if (cancelled) return
        update({
          snapshot: null,
          previewError: mapApiError(err, 'Failed to load revision preview'),
        })
      } finally {
        if (!cancelled) {
          update({ previewLoading: false })
        }
        useProgressbarStore.getState().setLoading(false)
      }
    }

    void loadSnapshot()

    return () => {
      cancelled = true
    }
  }, [activeTab, pageId, selectedRevisionId, update])

  useEffect(() => {
    if (
      !pageId ||
      !selectedRevisionId ||
      !latestRevisionId ||
      (activeTab !== 'changes' && activeTab !== 'assets')
    ) {
      return
    }

    let cancelled = false

    const loadComparison = async () => {
      useProgressbarStore.getState().setLoading(true)
      update({
        compareLoading: true,
        previewError: null,
      })
      try {
        const data = await compareRevisions(
          pageId,
          selectedRevisionId,
          latestRevisionId,
        )
        if (cancelled) return
        update({ comparison: data })
      } catch (err) {
        if (cancelled) return
        update({
          comparison: null,
          previewError: mapApiError(err, 'Failed to compare revisions'),
        })
      } finally {
        if (!cancelled) {
          update({ compareLoading: false })
        }
        useProgressbarStore.getState().setLoading(false)
      }
    }

    void loadComparison()

    return () => {
      cancelled = true
    }
  }, [activeTab, latestRevisionId, pageId, selectedRevisionId, update])
}

export async function loadMorePageHistory() {
  const state = usePageHistoryStore.getState()
  if (!state.pageId || !state.nextCursor || state.loadingMore) return

  state.update({
    loadingMore: true,
    listError: null,
  })

  try {
    const data = await listRevisions(state.pageId, state.nextCursor)
    const currentRevisions = usePageHistoryStore.getState().revisions
    const visibleRevisions = excludeLatestRevision(
      data.revisions,
      usePageHistoryStore.getState().latestRevisionId,
    )
    usePageHistoryStore.getState().update({
      revisions: [...currentRevisions, ...visibleRevisions],
      nextCursor: data.nextCursor,
    })
  } catch (err) {
    usePageHistoryStore.getState().update({
      listError: mapApiError(err, 'Failed to load more revisions'),
    })
  } finally {
    usePageHistoryStore.getState().update({
      loadingMore: false,
    })
  }
}

export async function reloadPageHistory(pageId: string) {
  await loadPageHistoryState(pageId, usePageHistoryStore.getState().update)
}
