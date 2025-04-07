import { filterTreeWithOpenNodes } from '@/lib/filterTreeWithOpenNodes'
import { useDebounce } from '@/lib/useDebounce'
import { useTreeStore } from '@/stores/tree'
import React, { useEffect } from 'react'
import { AddPageDialog } from '../page/AddPageDialog'
import { SortPagesDialog } from '../page/SortPagesDialog'
import { TreeNode } from './TreeNode'

export default function TreeView() {
  const {
    tree,
    loading,
    error,
    reloadTree,
    searchQuery,
    setSearchQuery,
    clearSearch,
  } = useTreeStore()

  const debouncedSearchQuery = useDebounce(searchQuery, 500);  // Debounce für 500ms

  useEffect(() => {
    reloadTree()
  }, [])

  useEffect(() => {
    if (!tree || !debouncedSearchQuery) return
    const { expandedIds } = filterTreeWithOpenNodes(tree, debouncedSearchQuery)
    useTreeStore.setState({ openNodeIds: expandedIds })
  }, [debouncedSearchQuery, tree])

  useEffect(() => {
    setSearchQuery(debouncedSearchQuery);
  }, [debouncedSearchQuery, setSearchQuery]);



  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error || !tree)
    return <p className="text-sm text-red-500">Error: {error}</p>

  const { filtered: filteredTree } = filterTreeWithOpenNodes(tree, debouncedSearchQuery)

  let toRender = <></>

  if (
    debouncedSearchQuery &&
    (!filteredTree?.children || filteredTree.children.length === 0)
  ) {
    toRender = (
      <p className="mt-2 text-sm italic text-gray-500">
        No pages found matching "{debouncedSearchQuery}"
      </p>
    )
  } else {
    toRender = (
      
      <div className="space-y-1 mt-4">
        <div>
          <AddPageDialog parentId={''} minimal />
          {filteredTree !== null && <SortPagesDialog parent={filteredTree} />}
        </div>
        {filteredTree?.children.map((node) => (
          <React.Fragment key={node.id}>
            <TreeNode node={node} />
          </React.Fragment>
        ))}
      </div>
    )
  }

  return (
    <>
      <div className='flex items-center space-x-2'>
        <input
          type="text"
          placeholder="Search pages..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full rounded border px-2 py-1 text-base"
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
