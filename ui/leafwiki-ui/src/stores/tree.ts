import { fetchTree, PageNode } from '@/lib/api/pages'
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

function buildIndexes(root: PageNode) {
  const byPath: Record<string, PageNode> = {}
  const byId: Record<string, PageNode> = {}

  const walk = (n: PageNode) => {
    byId[n.id] = n
    byPath[n.path] = n
    for (const ch of n.children || []) walk(ch)
  }

  walk(root)
  return { byPath, byId }
}

function assignParentIds(node: PageNode, parentId: string | null = null) {
  node.parentId = parentId
  for (const child of node.children || []) {
    assignParentIds(child, node.id)
  }
}

function toSetRecord(ids: string[]): Record<string, true> {
  const rec: Record<string, true> = {}
  for (const id of ids) rec[id] = true
  return rec
}

type TreeStore = {
  tree: PageNode | null
  loading: boolean
  error: string | null
  reloadTree: () => Promise<void>
  toggleNode: (id: string) => void
  openNode: (id: string) => void
  closeNode: (id: string) => void
  isNodeOpen: (id: string) => boolean
  getPageById: (id: string) => PageNode | null
  getPageByPath: (path: string) => PageNode | null
  getPathById: (id: string) => string | null
  getAncestors: (id: string) => string[]
  openAncestorsForPath: (path: string) => void
  openNodeIds: string[]
  openNodeIdSet: Record<string, true>
  byPath: Record<string, PageNode>
  byId: Record<string, PageNode>
}
export const useTreeStore = create<TreeStore>()(
  persist(
    (set, get) => ({
      tree: null,
      loading: false,
      error: null,
      openNodeIds: [],
      openNodeIdSet: {},
      byPath: {},
      byId: {},

      toggleNode: (id: string) => {
        const current = new Set(get().openNodeIds)

        if (current.has(id)) current.delete(id)
        else current.add(id)

        const ids = Array.from(current)
        set({ openNodeIds: ids, openNodeIdSet: toSetRecord(ids) })
      },

      openNode: (id: string) => {
        const current = new Set(get().openNodeIds)
        current.add(id)
        const ids = Array.from(current)
        set({ openNodeIds: ids, openNodeIdSet: toSetRecord(ids) })
      },

      closeNode: (id: string) => {
        const current = new Set(get().openNodeIds)
        current.delete(id)
        const ids = Array.from(current)
        set({ openNodeIds: ids, openNodeIdSet: toSetRecord(ids) })
      },

      isNodeOpen: (id: string) => !!get().openNodeIdSet?.[id],

      getPageByPath: (path: string) => get().byPath?.[path] ?? null,
      getPageById: (id: string) => get().byId?.[id] ?? null,
      getPathById: (id: string) => get().byId?.[id]?.path ?? null,

      getAncestors: (id: string) => {
        const byId = get().byId
        const out: string[] = []
        let cur = byId?.[id]
        while (cur?.parentId) {
          out.unshift(cur.parentId)
          cur = byId[cur.parentId]
        }
        return out
      },

      openAncestorsForPath: (path: string) => {
        const node = get().getPageByPath(path)
        if (!node) return

        const ancestors = get().getAncestors(node.id)
        if (ancestors.length === 0) return

        const merged = new Set(get().openNodeIds)
        for (const id of ancestors) merged.add(id)

        const ids = Array.from(merged)
        set({ openNodeIds: ids, openNodeIdSet: toSetRecord(ids) })
      },

      reloadTree: async () => {
        set({ loading: true, error: null })

        try {
          const tree = await fetchTree()
          assignParentIds(tree)
          const { byPath, byId } = buildIndexes(tree)
          const persistedOpen = get().openNodeIds
          set({ tree, byPath, byId, openNodeIdSet: toSetRecord(persistedOpen) })
          // FIXME: a better error handling is required here
        } catch (err: unknown) {
          if (err instanceof Error) {
            set({ error: err.message })
          } else {
            set({ error: 'An unknown error occurred' })
          }
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
