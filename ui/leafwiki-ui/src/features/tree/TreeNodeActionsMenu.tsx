import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  convertPage,
  NODE_KIND_PAGE,
  NODE_KIND_SECTION,
  PageNode,
} from '@/lib/api/pages'
import {
  DIALOG_ADD_PAGE,
  DIALOG_DELETE_PAGE_CONFIRMATION,
  DIALOG_MOVE_PAGE,
  DIALOG_SORT_PAGES,
} from '@/lib/registries'
import { useAppMode } from '@/lib/useAppMode'
import { getDeleteRedirectRoutePath } from '@/lib/wikiPath'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import {
  FilePlus,
  FolderPlus,
  List,
  MoreVertical,
  Move,
  Pencil,
  Repeat2,
  Trash,
} from 'lucide-react'
import { useCallback } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { toast } from 'sonner'
import { usePageEditorStore } from '../editor/pageEditor'
import { TreeViewActionButton } from './TreeViewActionButton'
import { useTreeNodeActionsMenusStore } from './treeNodeActionsMenus'

export type TreeNodeActionsMenuProps = {
  node: PageNode
}

export default function TreeNodeActionsMenu({
  node,
}: TreeNodeActionsMenuProps) {
  const { id: nodeId, kind: nodeKind, children } = node
  const appMode = useAppMode()
  const currentEditorPageId = usePageEditorStore((state) => state.page?.id)
  const openDialog = useDialogsStore((state) => state.openDialog)
  const reloadTree = useTreeStore((state) => state.reloadTree)
  const hasChildren = children && children.length > 0
  const navigate = useNavigate()
  const location = useLocation()
  const setOpenMenuNodeId = useTreeNodeActionsMenusStore(
    (s) => s.setOpenMenuNodeId,
  )
  const open = useTreeNodeActionsMenusStore((s) => s.openMenuNodeId === node.id)

  const handleConvertPage = useCallback(() => {
    convertPage(
      nodeId,
      nodeKind === NODE_KIND_PAGE ? NODE_KIND_SECTION : NODE_KIND_PAGE,
    )
      .then(() => {
        toast.success('Page converted successfully')
        reloadTree()
      })
      .catch(() => {
        toast.error('Failed to convert page')
      })
  }, [nodeId, nodeKind, reloadTree])

  const redirectUrlAfterDelete = useCallback(() => {
    return getDeleteRedirectRoutePath(location.pathname, node.path)
  }, [location.pathname, node.path])

  const isCurrentlyEditedNode =
    appMode === 'edit' && currentEditorPageId === node.id

  return (
    <DropdownMenu
      open={open}
      onOpenChange={(nextOpen) => setOpenMenuNodeId(nextOpen ? node.id : null)}
    >
      <DropdownMenuTrigger aria-label="More actions">
        <TreeViewActionButton
          actionName="open-more-actions"
          icon={<MoreVertical size={18} className="tree-node__action-icon" />}
          tooltip="Open more actions"
        />
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuItem
          className="cursor-pointer"
          onClick={() => {
            openDialog(DIALOG_ADD_PAGE, {
              parentId: nodeId,
              nodeKind: NODE_KIND_PAGE,
            })
          }}
        >
          <FilePlus size={18} className="tree-node__action-icon" /> Add Page
        </DropdownMenuItem>
        <DropdownMenuItem
          className="cursor-pointer"
          onClick={() => {
            openDialog(DIALOG_ADD_PAGE, {
              parentId: nodeId,
              nodeKind: NODE_KIND_SECTION,
            })
          }}
        >
          <FolderPlus size={18} className="tree-node__action-icon" /> Add
          Section
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="cursor-pointer"
          onClick={() => {
            navigate(`/e/${node.path}`)
          }}
        >
          <Pencil size={18} className="tree-node__action-icon" /> Edit{' '}
          {nodeKind === NODE_KIND_PAGE ? 'Page' : 'Section'}
        </DropdownMenuItem>
        {nodeKind === NODE_KIND_SECTION && hasChildren && (
          <DropdownMenuItem
            className="cursor-pointer"
            data-testid="tree-view-action-button-sort"
            onClick={() => openDialog(DIALOG_SORT_PAGES, { parent: node })}
          >
            <List size={18} className="tree-node__action-icon" /> Sort Section
          </DropdownMenuItem>
        )}
        <DropdownMenuItem
          className="cursor-pointer"
          data-testid="tree-view-action-button-move"
          onClick={() => openDialog(DIALOG_MOVE_PAGE, { pageId: node.id })}
        >
          <Move size={18} className="tree-node__action-icon" /> Move{' '}
          {nodeKind === NODE_KIND_PAGE ? 'Page' : 'Section'}
        </DropdownMenuItem>
        {nodeKind === NODE_KIND_SECTION && !hasChildren && (
          <DropdownMenuItem
            className="cursor-pointer"
            onClick={handleConvertPage}
          >
            <Repeat2 size={18} className="tree-node__action-icon" /> Convert to
            Page
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem
          className="text-error cursor-pointer"
          data-testid="tree-view-action-button-delete"
          onClick={() => {
            if (isCurrentlyEditedNode) {
              toast.warning(
                `This ${nodeKind === NODE_KIND_PAGE ? 'page' : 'section'} is currently being edited. Please close the editor before deleting it.`,
              )
              return
            }

            openDialog(DIALOG_DELETE_PAGE_CONFIRMATION, {
              pageId: node?.id,
              redirectTo: redirectUrlAfterDelete(),
            })
          }}
        >
          <Trash size={18} className="tree-node__action-icon text-error" />{' '}
          Delete {nodeKind === NODE_KIND_PAGE ? 'Page' : 'Section'}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
