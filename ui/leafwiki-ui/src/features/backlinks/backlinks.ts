// zustands store to fetch the backlinks of a page

import { fetchBacklinks, type Backlink } from '@/lib/api/backlinks'
import { create } from 'zustand'

type BacklinksStore = {
  backlinks: Backlink[]
  count: number
  loading: boolean
  error: string | null
  fetchPageBacklinks: (pageId: string) => Promise<void>
}

export const useBacklinksStore = create<BacklinksStore>((set) => ({
  backlinks: [],
  count: 0,
  loading: false,
  error: null,

  fetchPageBacklinks: async (pageId: string) => {
    if (!pageId) {
      set({
        backlinks: [],
        count: 0,
        loading: false,
        error: 'Page ID is required',
      })
      return
    }

    set({ loading: true, error: null })
    try {
      const data = await fetchBacklinks(pageId)
      set({ backlinks: data.backlinks, count: data.count, loading: false })
    } catch (error: unknown) {
      if (error instanceof Error) {
        set({ error: error.message, loading: false })
      } else {
        set({ error: 'Failed to fetch backlinks', loading: false })
      }
    }
  },
}))
