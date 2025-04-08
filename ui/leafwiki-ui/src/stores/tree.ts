import { fetchTree, PageNode } from '@/lib/api'
import { create } from 'zustand'

type TreeStore = {
  tree: PageNode | null
  loading: boolean
  error: string | null
  searchQuery: string
  setSearchQuery: (query: string) => void
  clearSearch: () => void
  reloadTree: () => Promise<void>
  toggleNode: (id: string) => void
  isNodeOpen: (id: string) => boolean
  getPageById: (id: string) => PageNode | null
  getPathById: (id: string) => string | null
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

  searchQuery: '',

  setSearchQuery: (query: string) => {
    set({ searchQuery: query })
  },

  clearSearch: () => {
    set({ searchQuery: '' })
  },

  getPathById: (id: string) => {
    const findNodeById = (node: PageNode): PageNode | null => {
      if (node.id === id) return node
      for (const child of node.children || []) {
        const found = findNodeById(child)
        if (found) return found
      }
      return null
    }

    const tree = get().tree
    if (!tree) return null

    const node = findNodeById(tree)
    return node?.path ?? null
  },

  getPageById: (id: string) => {
    const findNodeById = (node: PageNode): PageNode | null => {
      if (node.id === id) return node
      for (const child of node.children || []) {
        const found = findNodeById(child)
        if (found) return found
      }
      return null
    }

    const tree = get().tree
    if (!tree) return null

    const node = findNodeById(tree)
    return node ?? null
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
