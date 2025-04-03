import { filterTreeWithOpenNodes } from "@/lib/filterTreeWithOpenNodes"
import { useTreeStore } from '@/stores/tree'
import React, { useEffect } from 'react'
import { SortPagesButton } from "../page/SortPagesButton"
import { TreeAddInline } from "./TreeAddInline"
import { TreeNode } from "./TreeNode"

export default function TreeView() {
  const { tree, loading, error, reloadTree, searchQuery, setSearchQuery, clearSearch } = useTreeStore()

  useEffect(() => {
    reloadTree()
  }, [])

  useEffect(() => {
    if (!tree || !searchQuery) return
    const { expandedIds } = filterTreeWithOpenNodes(tree, searchQuery)
    useTreeStore.setState({ openNodeIds: expandedIds })
  }, [searchQuery, tree])

  useEffect(() => {
    if (!searchQuery) {
      useTreeStore.setState({ openNodeIds: new Set() })
    }
  }, [searchQuery])

  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error || !tree)
    return <p className="text-sm text-red-500">Error: {error}</p>

  const { filtered: filteredTree } = filterTreeWithOpenNodes(tree, searchQuery)

  let toRender = <></>

  if (searchQuery && (!filteredTree?.children || filteredTree.children.length === 0)) {
    toRender = (
      <p className="text-sm text-gray-500 italic px-2">Keine Treffer gefunden</p>
    )
  } else {
    toRender = (<div className="space-y-1">
      {filteredTree?.children.map((node) => (
        <React.Fragment key={node.id}>
          <TreeNode node={node} />
        </React.Fragment>
      ))}
      <div className="ml-2">
        <TreeAddInline parentId={''} minimal />
        {filteredTree !== null && <SortPagesButton parent={filteredTree} />}
      </div>
    </div>)
  }

  return (
    <>
      <div className="mb-2 flex gap-2">
        <input
          type="text"
          placeholder="Search pages..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full rounded border px-2 py-1 text-sm"
        />
        {searchQuery && (
          <button
            onClick={clearSearch}
            className="text-xs text-gray-500 hover:underline"
          >
            Clear
          </button>
        )}
      </div>
      {toRender}
    </>
  )
}
