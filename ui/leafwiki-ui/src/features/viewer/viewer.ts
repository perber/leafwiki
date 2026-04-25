// store to manage the viewer state
// f.g. loading, error, page data

import { getPageByPath, Page } from '@/lib/api/pages'
import { isPageNotFoundError } from '@/lib/api/errors'
import { create } from 'zustand'
import { useProgressbarStore } from '../progressbar/progressbar'

interface ViewerState {
  error: string | null
  notFound: boolean
  page: Page | null
  setError: (error: string | null) => void
  clear: () => void
  loadPageData: (path: string) => Promise<void>
}

export const useViewerStore = create<ViewerState>((set) => ({
  error: null,
  notFound: false,
  page: null,
  setError: (error) => set({ error }),
  clear: () => set({ error: null, notFound: false, page: null }),
  loadPageData: async (path: string) => {
    useProgressbarStore.getState().setLoading(true)
    set({ error: null, notFound: false, page: null })
    try {
      const page = await getPageByPath(path)
      set({ page, notFound: false })
    } catch (err) {
      if (isPageNotFoundError(err)) {
        set({ error: null, notFound: true, page: null })
      } else if (err instanceof Error) {
        set({ error: err.message, notFound: false })
      } else {
        set({ error: 'An unknown error occurred', notFound: false })
      }
    } finally {
      useProgressbarStore.getState().setLoading(false)
    }
  },
}))
