// stores/sidebarPanels.ts
// Tracks which sidebar accordion sections (Pinned Pages, Pages, ...) are
// expanded. The state is persisted across sessions using localStorage.

import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type SidebarPanelsStore = {
  openSections: string[]
  setOpenSections: (ids: string[]) => void
}

export const useSidebarPanelsStore = create<SidebarPanelsStore>()(
  persist(
    (set) => ({
      openSections: ['pinned', 'pages'],
      setOpenSections: (ids) => set({ openSections: ids }),
    }),
    {
      name: 'leafwiki-sidebar-panels', // localStorage key
    },
  ),
)
