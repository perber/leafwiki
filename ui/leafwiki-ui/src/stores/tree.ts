import { fetchTree, PageNode } from "@/lib/api"
import { create } from "zustand"

type TreeStore = {
  tree: PageNode | null
  loading: boolean
  error: string | null
  reloadTree: () => Promise<void>
  toggleNode: (id: string) => void
  isNodeOpen: (id: string) => boolean
  openNodeIds: Set<string>
}

export const useTreeStore = create<TreeStore>((set, get) => ({
  tree: null,
  loading: false,
  error: null,
  openNodeIds: new Set<string>(),

  toggleNode: (id: string) => {
    const openNodeIds = new Set(get().openNodeIds)
    if (openNodeIds.has(id)) {
      openNodeIds.delete(id)
    } else {
      openNodeIds.add(id)
    }
    set({ openNodeIds })
  },

  isNodeOpen: (id: string) => get().openNodeIds.has(id),

  reloadTree: async () => {
    set({ loading: true, error: null })

    try {
      const tree = await fetchTree()
      set({ tree })
    } catch (err: any) {
      set({ error: err.message })
    } finally {
      set({ loading: false })
    }
  },

}))