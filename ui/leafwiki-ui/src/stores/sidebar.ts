// stores/sidebar.ts
// This store manages sidebar state: visibility and active mode (tree/search).
// The state is persisted across sessions using localStorage.

import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export const DEFAULT_SIDEBAR_WIDTH = 345
export const MIN_SIDEBAR_WIDTH = 220
export const MAX_SIDEBAR_WIDTH = 800

type SidebarStore = {
  sidebarMode: string
  setSidebarMode: (mode: string) => void

  sidebarVisible: boolean
  setSidebarVisible: (visible: boolean) => void

  sidebarWidth: number
  setSidebarWidth: (width: number) => void
}

export const useSidebarStore = create<SidebarStore>()(
  persist(
    (set) => ({
      sidebarMode: 'tree',
      setSidebarMode: (mode) => set({ sidebarMode: mode }),

      sidebarVisible: false,
      setSidebarVisible: (visible) => set({ sidebarVisible: visible }),

      sidebarWidth: DEFAULT_SIDEBAR_WIDTH,
      setSidebarWidth: (width) => {
        const clamped = Math.min(
          MAX_SIDEBAR_WIDTH,
          Math.max(MIN_SIDEBAR_WIDTH, width),
        )
        set({ sidebarWidth: clamped })
      },
    }),
    {
      name: 'leafwiki-sidebar', // localStorage key
      partialize: (state) => ({
        sidebarVisible: state.sidebarVisible,
        sidebarMode: state.sidebarMode,
        sidebarWidth: state.sidebarWidth,
      }),
    },
  ),
)
