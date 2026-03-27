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

export type HistoryMode = 'preview' | 'compare'

type PageHistoryState = {
  pageId: string
  revisions: Revision[]
  selectedRevisionId: string | null
  latestRevisionId: string | null
  snapshot: RevisionSnapshot | null
  comparison: RevisionComparison | null
  mode: HistoryMode
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
  setMode: (mode: HistoryMode) => void
}

const initialState: PageHistoryState = {
  pageId: '',
  revisions: [],
  selectedRevisionId: null,
  latestRevisionId: null,
  snapshot: null,
  comparison: null,
  mode: 'preview',
  listLoading: false,
  previewLoading: false,
  compareLoading: false,
  listError: null,
  previewError: null,
  nextCursor: '',
  loadingMore: false,
}

export const usePageHistoryStore = create<PageHistoryStore>((set) => ({
  ...initialState,
  update: (patch) => set((state) => ({ ...state, ...patch })),
  reset: () => set(initialState),
  selectRevision: (revisionId) =>
    set({
      selectedRevisionId: revisionId,
      previewError: null,
      snapshot: null,
      comparison: null,
    }),
  setMode: (mode) => set({ mode }),
}))

export function usePageHistory(pageId: string | null) {
  const update = usePageHistoryStore((state) => state.update)
  const reset = usePageHistoryStore((state) => state.reset)
  const selectedRevisionId = usePageHistoryStore(
    (state) => state.selectedRevisionId,
  )
  const latestRevisionId = usePageHistoryStore(
    (state) => state.latestRevisionId,
  )
  const mode = usePageHistoryStore((state) => state.mode)

  useEffect(() => {
    if (!pageId) {
      reset()
      return
    }

    let cancelled = false

    const load = async () => {
      update({
        pageId,
        revisions: [],
        selectedRevisionId: null,
        latestRevisionId: null,
        snapshot: null,
        comparison: null,
        mode: 'preview',
        listLoading: true,
        previewLoading: false,
        compareLoading: false,
        listError: null,
        previewError: null,
        nextCursor: '',
        loadingMore: false,
      })

      try {
        const [historyData, latestRevision] = await Promise.all([
          listRevisions(pageId),
          getLatestRevision(pageId),
        ])
        if (cancelled) return

        const firstRevision = historyData.revisions[0] ?? null
        update({
          revisions: historyData.revisions,
          nextCursor: historyData.nextCursor,
          latestRevisionId: latestRevision.id,
          selectedRevisionId: firstRevision?.id ?? null,
        })
      } catch (err) {
        if (cancelled) return
        update({
          listError: mapApiError(err, 'Failed to load page history'),
          revisions: [],
          nextCursor: '',
          latestRevisionId: null,
        })
      } finally {
        if (!cancelled) {
          update({ listLoading: false })
        }
      }
    }

    void load()

    return () => {
      cancelled = true
      reset()
    }
  }, [pageId, reset, update])

  useEffect(() => {
    if (!pageId || !selectedRevisionId || mode !== 'preview') return

    let cancelled = false

    const loadSnapshot = async () => {
      update({
        previewLoading: true,
        previewError: null,
        comparison: null,
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
      }
    }

    void loadSnapshot()

    return () => {
      cancelled = true
    }
  }, [mode, pageId, selectedRevisionId, update])

  useEffect(() => {
    if (
      !pageId ||
      !selectedRevisionId ||
      !latestRevisionId ||
      mode !== 'compare'
    ) {
      return
    }

    let cancelled = false

    const loadComparison = async () => {
      update({
        compareLoading: true,
        previewError: null,
        snapshot: null,
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
      }
    }

    void loadComparison()

    return () => {
      cancelled = true
    }
  }, [latestRevisionId, mode, pageId, selectedRevisionId, update])
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
    usePageHistoryStore.getState().update({
      revisions: [...currentRevisions, ...data.revisions],
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
