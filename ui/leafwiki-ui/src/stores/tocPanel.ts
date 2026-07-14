// stores/tocPanel.ts
// Tracks whether the right-hand "On this page" TOC pane is collapsed.
// The state is persisted across sessions using localStorage.

import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type TocPanelStore = {
  collapsed: boolean
  setCollapsed: (collapsed: boolean) => void
  toggleCollapsed: () => void
}

export const useTocPanelStore = create<TocPanelStore>()(
  persist(
    (set) => ({
      collapsed: false,
      setCollapsed: (collapsed) => set({ collapsed }),
      toggleCollapsed: () => set((state) => ({ collapsed: !state.collapsed })),
    }),
    {
      name: 'leafwiki-toc-panel', // localStorage key
    },
  ),
)
