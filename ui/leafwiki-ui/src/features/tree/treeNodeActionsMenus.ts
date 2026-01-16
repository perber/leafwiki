// Tree node actions menus store
// used to manage the open state of tree node action menus in the application

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
