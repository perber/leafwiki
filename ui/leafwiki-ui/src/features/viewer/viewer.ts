// store to manage the viewer state
// f.g. loading, error, page data

import { getPageByPath, Page } from '@/lib/api/pages'
import { isPageNotFoundError } from '@/lib/api/errors'
import { create } from 'zustand'
import { useProgressbarStore } from '../progressbar/progressbarStore'

interface ViewerState {
  error: string | null
  isLoading: boolean
  notFound: boolean
  page: Page | null
  setError: (error: string | null) => void
  clear: () => void
  loadPageData: (path: string) => Promise<void>
}

let loadController: AbortController | null = null

export const useViewerStore = create<ViewerState>((set) => ({
  error: null,
  isLoading: false,
  notFound: false,
  page: null,
  setError: (error) => set({ error }),
  clear: () => {
    loadController?.abort()
    loadController = null
    set({ error: null, isLoading: false, notFound: false, page: null })
  },
  loadPageData: async (path: string) => {
    loadController?.abort()
    loadController = new AbortController()
    const { signal } = loadController

    useProgressbarStore.getState().setLoading(true)
    set({ error: null, isLoading: true, notFound: false })
    try {
      const page = await getPageByPath(path, signal)
      set({ page, notFound: false })
    } catch (err) {
      if (signal.aborted) return

      if (isPageNotFoundError(err)) {
        set({ error: null, notFound: true, page: null })
      } else if (err instanceof Error) {
        set({ error: err.message, notFound: false })
      } else {
        set({ error: 'An unknown error occurred', notFound: false })
      }
    } finally {
      if (!signal.aborted) {
        set({ isLoading: false })
        useProgressbarStore.getState().setLoading(false)
      }
    }
  },
}))
