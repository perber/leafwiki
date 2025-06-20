import { TreeViewActionButton } from '@/components/TreeViewActionButton'
import { getAncestorIds } from '@/lib/treeUtils'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { List, Plus } from 'lucide-react'
import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'
import { TreeNode } from './TreeNode'

export default function TreeView() {
  const tree = useTreeStore((s) => s.tree)
  const loading = useTreeStore((s) => s.loading)
  const error = useTreeStore((s) => s.error)
  const reloadTree = useTreeStore((s) => s.reloadTree)

  const location = useLocation()
  const currentPath = location.pathname.replace(/^\/(e\/)?/, '') // z.B. docs/setup/intro

  const openDialog = useDialogsStore((state) => state.openDialog)
  const readOnlyMode = useIsReadOnly()

  useEffect(() => {
    if (!tree || !currentPath) return

    const page = useTreeStore.getState().getPageByPath(currentPath)
    if (page) {
      const ancestors = getAncestorIds(tree, page.id)
      useTreeStore.setState((state) => ({
        openNodeIds: Array.from(new Set([...state.openNodeIds, ...ancestors])),
      }))
    }
  }, [tree, currentPath])

  useEffect(() => {
    if (tree === null) {
      reloadTree()
    }
  }, [tree, reloadTree])

  if (loading) return <p className="text-sm text-gray-500">Loading...</p>
  if (error || !tree)
    return <p className="text-sm text-red-500">Error: {error}</p>

  return (
    <>
      <div className="mb-2 mt-2">
        {!readOnlyMode && (
          <div className="mb-1 flex">
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
            {tree !== null && (
              <TreeViewActionButton
                icon={
                  <List
                    size={20}
                    className="cursor-pointer text-gray-500 hover:text-gray-800"
                  />
                }
                tooltip="Sort pages"
                onClick={() => openDialog('sort', { parent: tree })}
              />
            )}
          </div>
        )}
        {tree?.children.map((node) => <TreeNode key={node.id} node={node} />)}
      </div>
    </>
  )
}
