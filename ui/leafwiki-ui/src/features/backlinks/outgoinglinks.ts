// zustands store to fetch the outgoing links of a page

import { fetchOutgoingLinks, OutgoingLinks } from '@/lib/api/backlinks'
import { create } from 'zustand'

type OutgoingLinksStore = {
  outgoing: OutgoingLinks[]
  count: number
  loading: boolean
  error: string | null
  fetchPageOutgoingLinks: (pageId: string) => Promise<void>
}

export const useOutgoingLinksStore = create<OutgoingLinksStore>((set) => ({
  outgoing: [],
  count: 0,
  loading: false,
  error: null,

  fetchPageOutgoingLinks: async (pageId: string) => {
    if (!pageId) {
      set({
        outgoing: [],
        count: 0,
        loading: false,
        error: 'Page ID is required',
      })
      return
    }

    set({ loading: true, error: null })
    try {
      const data = await fetchOutgoingLinks(pageId)
      console.log(data)
      set({ outgoing: data.outgoings, count: data.count, loading: false })
    } catch (error: unknown) {
      if (error instanceof Error) {
        set({ error: error.message, loading: false })
      } else {
        set({ error: 'Failed to fetch outgoing links', loading: false })
      }
    }
  },
}))
