// store to manage the viewer state
// f.g. loading, error, page data

import { getPageByPath, Page } from '@/lib/api/pages'
import { create } from 'zustand'

interface ViewerState {
  loading: boolean
  error: string | null
  page: Page | null
  setLoading: (loading: boolean) => void
  setError: (error: string | null) => void
  loadPageData?: (path: string) => Promise<void>
}

export const useViewerStore = create<ViewerState>((set) => ({
  loading: true,
  error: null,
  page: null,
  setLoading: (loading) => set({ loading }),
  setError: (error) => set({ error }),
  loadPageData: async (path: string) => {
    set({ loading: true, error: null })
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
      set({ loading: false })
    }
  },
}))
