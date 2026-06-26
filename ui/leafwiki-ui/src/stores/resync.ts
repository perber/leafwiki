import { create } from 'zustand'
import { triggerResync } from '@/lib/api/resync'

interface ResyncState {
  isLoading: boolean
  trigger: () => Promise<void>
}

export const useResyncStore = create<ResyncState>((set) => ({
  isLoading: false,

  trigger: async () => {
    set({ isLoading: true })
    try {
      await triggerResync()
    } finally {
      set({ isLoading: false })
    }
  },
}))
