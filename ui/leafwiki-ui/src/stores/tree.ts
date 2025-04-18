import { fetchTree, PageNode } from '@/lib/api'
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

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
  getPageByPath: (path: string) => PageNode | null
  getPathById: (id: string) => string | null
  openNodeIds: string[]
  prevOpenNodeIds: string[] | null
}
export const useTreeStore = create<TreeStore>()(
  persist(
    (set, get) => ({
      tree: null,
      loading: false,
      error: null,
      prevOpenNodeIds: null,
      openNodeIds: [],

      toggleNode: (id: string) => {
        const current = new Set(get().openNodeIds)

        if (current.has(id)) {
          current.delete(id)
        } else {
          current.add(id)
        }

        if (current.size === 0) {
          set({ openNodeIds: [] })
          return
        }

        set({ openNodeIds: Array.from(current) })
      },

      searchQuery: '',

      setSearchQuery: (query: string) => {
        const wasEmpty = get().searchQuery === ''
        const isNowEmpty = query === ''

        if (wasEmpty && !isNowEmpty) {
          set({ prevOpenNodeIds: get().openNodeIds })
        }

        if (!wasEmpty && isNowEmpty) {
          const previous = get().prevOpenNodeIds
          if (previous) {
            set({ openNodeIds: previous, prevOpenNodeIds: null })
          }
        }

        set({ searchQuery: query })
      },

      clearSearch: () => {
        const previous = get().prevOpenNodeIds
        if (previous) {
          set({ openNodeIds: previous, prevOpenNodeIds: null })
        }
        set({ searchQuery: '' })
      },

      isNodeOpen: (id: string) => {
        return get().openNodeIds.includes(id)
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

      getPageByPath: (path: string) => {
        const findNodeByPath = (node: PageNode): PageNode | null => {
          if (node.path === path) return node
          for (const child of node.children || []) {
            const found = findNodeByPath(child)
            if (found) return found
          }
          return null
        }

        const tree = get().tree
        if (!tree) return null

        return findNodeByPath(tree)
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

        return findNodeById(tree)
      },

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
    }),
    {
      name: 'leafwiki-tree-open-node-ids',
      partialize: (state) => ({
        openNodeIds: state.openNodeIds,
      }),
    },
  ),
)
