// SidebarStore
// This file is used to manage the state of the sidebar in the application.
// Currently is just holds the state between search and tree view.

import { create } from 'zustand'

type SidebarStore = {
  sidebarMode: string // 'tree' | 'search'
  setSidebarMode: (mode: string) => void
}

export const useSidebarStore = create<SidebarStore>((set) => ({
  sidebarMode: 'tree', // Default mode is 'tree'
  setSidebarMode: (mode: string) => set({ sidebarMode: mode }),
}))
