import { TreeViewActionButton } from '@/features/tree/TreeViewActionButton'
import { NODE_KIND_PAGE, NODE_KIND_SECTION } from '@/lib/api/pages'
import { DIALOG_ADD_PAGE, DIALOG_SORT_PAGES } from '@/lib/registries'
import { getAncestorIds } from '@/lib/treeUtils'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { FilePlus, FolderPlus, List } from 'lucide-react'
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

  if (loading)
    return (
      <p className="tree-view__status tree-view__status--loading">Loading...</p>
    )

  if (error || !tree)
    return (
      <p className="tree-view__status tree-view__status--error">
        Error: {error}
      </p>
    )

  return (
    <div className="tree-view">
      {!readOnlyMode && (
        <div className="tree-view__toolbar">
          <TreeViewActionButton
            actionName="add"
            icon={<FilePlus size={18} className="tree-view__action-icon" />}
            tooltip="Create new page"
            onClick={() => openDialog(DIALOG_ADD_PAGE, { parentId: '', nodeKind: NODE_KIND_PAGE })}
          />
          <TreeViewActionButton
            actionName="add-section"
            icon={<FolderPlus size={20} className="tree-view__action-icon" />}
            tooltip="Create new section"
            onClick={() => openDialog(DIALOG_ADD_PAGE, { parentId: '', nodeKind: NODE_KIND_SECTION })}
          />
          {tree && (
            <TreeViewActionButton
              actionName="sort"
              icon={<List size={20} className="tree-view__action-icon" />}
              tooltip="Sort pages"
              onClick={() => openDialog(DIALOG_SORT_PAGES, { parent: tree })}
            />
          )}
        </div>
      )}
      <div className="tree-view__nodes">
        {tree?.children?.map((node) => (
          <TreeNode key={node.id} node={node} />
        ))}
      </div>
    </div>
  )
}
