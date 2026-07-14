import { createContext, useContext } from 'react'
import { DropTarget } from './treeDndUtils'

export type TreeDndState = {
  enabled: boolean
  activeId: string | null
  dropTarget: DropTarget | null
}

export const TreeDndContext = createContext<TreeDndState>({
  enabled: false,
  activeId: null,
  dropTarget: null,
})

export function useTreeDnd() {
  return useContext(TreeDndContext)
}
