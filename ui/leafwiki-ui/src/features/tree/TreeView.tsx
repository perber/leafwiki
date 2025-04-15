import { TreeViewActionButton } from '@/components/TreeViewActionButton'
import { filterTreeWithOpenNodes } from '@/lib/filterTreeWithOpenNodes'
import { useDebounce } from '@/lib/useDebounce'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { List, Plus } from 'lucide-react'
import React, { useEffect } from 'react'
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

  const openDialog = useDialogsStore((state) => state.openDialog)

  // Debounce für die Suche: Warte 500ms, bevor die Suche verarbeitet wird
  const debouncedSearchQuery = useDebounce(searchQuery, 500)

  // Lade die Baumstruktur bei Komponentemount
  useEffect(() => {
    if (tree === null) {
      reloadTree()
    }
  }, [tree, reloadTree])

  // Bei Änderung der Debounced-Suche: Setze den Filterstatus
  useEffect(() => {
    if (!tree || !debouncedSearchQuery) return
    const { expandedIds } = filterTreeWithOpenNodes(tree, debouncedSearchQuery)
    useTreeStore.setState({ openNodeIds: expandedIds })
  }, [debouncedSearchQuery, tree])

  // Verwende den debouncedSearchQuery für das Setzen des Such-Querys
  useEffect(() => {
    setSearchQuery(debouncedSearchQuery)
  }, [debouncedSearchQuery, setSearchQuery])

  // Fehlerbehandlung und Ladezustand
  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error || !tree)
    return <p className="text-sm text-red-500">Error: {error}</p>

  // Filterung mit dem debouncedSearchQuery
  const { filtered: filteredTree } = filterTreeWithOpenNodes(
    tree,
    debouncedSearchQuery,
  )

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
          <TreeViewActionButton icon={<Plus size={20} className="cursor-pointer text-gray-500 hover:text-gray-800" />} tooltip="Create new page" onClick={() => openDialog("add", {parentId: ""})} />
          {filteredTree !== null && <TreeViewActionButton icon={<List size={20} className="cursor-pointer text-gray-500 hover:text-gray-800" />} tooltip="Sort pages" onClick={() => openDialog("sort", { parent: filteredTree })} />}
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
      <div className="flex items-center space-x-2">
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
