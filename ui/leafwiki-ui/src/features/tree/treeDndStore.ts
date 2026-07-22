import { create } from 'zustand'
import { DropTarget } from './treeDndUtils'

type TreeDndState = {
  enabled: boolean
  activeId: string | null
  dropTarget: DropTarget | null
}

export const useTreeDndStore = create<TreeDndState>()(() => ({
  enabled: false,
  activeId: null,
  dropTarget: null,
}))
