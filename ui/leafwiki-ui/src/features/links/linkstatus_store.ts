import { fetchLinkStatus, type LinkStatusResult } from '@/lib/api/links'
import { create } from 'zustand'

type LinkStatusStore = {
  status: LinkStatusResult | null
  loading: boolean
  error: string | null
  fetchLinkStatusForPage: (pageId: string) => Promise<void>
  clear: () => void
}

export const useLinkStatusStore = create<LinkStatusStore>((set) => ({
  status: null,
  loading: false,
  error: null,

  clear: () => set({ status: null, loading: false, error: null }),

  fetchLinkStatusForPage: async (pageId: string) => {
    if (!pageId) {
      set({ status: null, loading: false, error: 'Page ID is required' })
      return
    }
    set({ loading: true, error: null })
    try {
      const data = await fetchLinkStatus(pageId)
      set({ status: data, loading: false })
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : 'Failed to fetch link status'
      set({ error: msg, loading: false })
    }
  },
}))
