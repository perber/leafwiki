// stores/sidebar.ts
// This store manages sidebar state: visibility and active mode (tree/search).
// The state is persisted across sessions using localStorage.

import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type SidebarStore = {
  sidebarMode: 'tree' | 'search'
  setSidebarMode: (mode: 'tree' | 'search') => void
  sidebarVisible: boolean
  setSidebarVisible: (visible: boolean) => void
}

export const useSidebarStore = create<SidebarStore>()(
  persist(
    (set) => ({
      sidebarMode: 'tree',
      setSidebarMode: (mode) => set({ sidebarMode: mode }),

      sidebarVisible: false,
      setSidebarVisible: (visible) => set({ sidebarVisible: visible }),
    }),
    {
      name: 'leafwiki-sidebar', // localStorage key
      partialize: (state) => ({
        sidebarVisible: state.sidebarVisible,
        sidebarMode: state.sidebarMode,
      }),
    }
  )
)
