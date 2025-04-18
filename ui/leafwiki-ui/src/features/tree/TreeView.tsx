import { TreeViewActionButton } from '@/components/TreeViewActionButton'
import { filterTreeWithOpenNodes } from '@/lib/filterTreeWithOpenNodes'
import { useDebounce } from '@/lib/useDebounce'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { List, Plus } from 'lucide-react'
import { startTransition, useEffect, useMemo, useState } from 'react'
import { TreeNode } from './TreeNode'

export default function TreeView() {
  const searchQuery = useTreeStore((s) => s.searchQuery)
  const tree = useTreeStore((s) => s.tree)
  const loading = useTreeStore((s) => s.loading)
  const error = useTreeStore((s) => s.error)
  const reloadTree = useTreeStore((s) => s.reloadTree)
  const setSearchQuery = useTreeStore((s) => s.setSearchQuery)
  const clearSearch = useTreeStore((s) => s.clearSearch)

  const openDialog = useDialogsStore((state) => state.openDialog)

  const [inputValue, setInputValue] = useState(searchQuery)

  const debouncedSearchQuery = useDebounce(inputValue, 300)

  console.log('TreeView', { inputValue, searchQuery, debouncedSearchQuery })

  useEffect(() => {
    if (tree === null) {
      reloadTree()
    }
  }, [tree, reloadTree])

  useEffect(() => {
    if (!tree || !debouncedSearchQuery) return
    const { expandedIds } = filterTreeWithOpenNodes(tree, debouncedSearchQuery)
    startTransition(() => {
      useTreeStore.setState({ openNodeIds: expandedIds })
    })
  }, [debouncedSearchQuery, tree])

  useEffect(() => {
    setSearchQuery(debouncedSearchQuery)
  }, [debouncedSearchQuery, setSearchQuery])

  // Fehlerbehandlung und Ladezustand
  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error || !tree)
    return <p className="text-sm text-red-500">Error: {error}</p>

  // Filterung mit dem debouncedSearchQuery
  const { filtered: filteredTree } = useMemo(() => {
    return filterTreeWithOpenNodes(
      tree,
      debouncedSearchQuery,
    )
  }, [tree, debouncedSearchQuery])

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
      <div className="mt-4 space-y-1">
        <div className="flex">
          <TreeViewActionButton
            icon={
              <Plus
                size={20}
                className="cursor-pointer text-gray-500 hover:text-gray-800"
              />
            }
            tooltip="Create new page"
            onClick={() => openDialog('add', { parentId: '' })}
          />
          {filteredTree !== null && (
            <TreeViewActionButton
              icon={
                <List
                  size={20}
                  className="cursor-pointer text-gray-500 hover:text-gray-800"
                />
              }
              tooltip="Sort pages"
              onClick={() => openDialog('sort', { parent: filteredTree })}
            />
          )}
        </div>
        {filteredTree?.children.map((node) => (
          <TreeNode key={node.id} node={node} />
        ))}
      </div>
    )
  }

  return (
    <>
      <div className="flex items-center space-x-2">
        <input
          type="text"
          placeholder="Search pages..."
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          className="w-full rounded border px-2 py-1 text-base"
        />
        {inputValue && (
          <button
            onClick={() => {
              setInputValue('')
              clearSearch()
            }}
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
