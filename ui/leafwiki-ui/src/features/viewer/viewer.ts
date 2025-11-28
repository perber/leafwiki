// store to manage the viewer state
// f.g. loading, error, page data

import { getPageByPath, Page } from '@/lib/api/pages'
import { create } from 'zustand'
import { useProgressbarStore } from '../progressbar/progressbar'

interface ViewerState {
  error: string | null
  page: Page | null
  setError: (error: string | null) => void
  loadPageData?: (path: string) => Promise<void>
}

export const useViewerStore = create<ViewerState>((set) => ({
  error: null,
  page: null,
  setError: (error) => set({ error }),
  loadPageData: async (path: string) => {
    useProgressbarStore.getState().setLoading(true)
    set({ error: null })
    try {
      const page = await getPageByPath(path)
      set({ page })
    } catch (err) {
      if (err instanceof Error) {
        set({ error: err.message })
      } else {
        set({ error: 'An unknown error occurred' })
      }
    } finally {
      useProgressbarStore.getState().setLoading(false)
    }
  },
}))
