// SidebarStore
// This file is used to manage the state of the sidebar in the application.
// Currently is just holds the state between search and tree view.

import { create } from 'zustand'

type SidebarStore = {
  sidebarMode: string // 'tree' | 'search'
  setSidebarMode: (mode: string) => void
  sidebarVisible?: boolean // Optional, can be used to control visibility
  setSidebarVisible?: (visible: boolean, userOverride: boolean) => void
  userOverride: boolean,
}

export const useSidebarStore = create<SidebarStore>((set) => ({
  sidebarMode: 'tree', // Default mode is 'tree'
  setSidebarMode: (mode: string) => set({ sidebarMode: mode }),
  sidebarVisible: false, // Default visibility is false
  setSidebarVisible: (visible: boolean, userOverride: boolean = true) => set({ sidebarVisible: visible, userOverride }),
  userOverride: false, // Default user override is false
}))
