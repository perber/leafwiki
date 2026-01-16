// Dialogs store
// is used to manage the state of dialogs in the application

import { create } from 'zustand'

type TreeNodeActionsMenusStore = {
  openMenuNodeId: string | null
  setOpenMenuNodeId: (nodeId: string | null) => void
}

export const useTreeNodeActionsMenusStore = create<TreeNodeActionsMenusStore>(
  (set) => ({
    openMenuNodeId: null,
    setOpenMenuNodeId: (nodeId: string | null) => {
      set({ openMenuNodeId: nodeId })
    },
  }),
)
