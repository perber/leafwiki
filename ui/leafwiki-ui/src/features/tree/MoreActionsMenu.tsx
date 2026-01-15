import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { convertPage, NODE_KIND_PAGE, NODE_KIND_SECTION, PageNode } from '@/lib/api/pages'
import {
  DIALOG_ADD_PAGE,
  DIALOG_DELETE_PAGE_CONFIRMATION,
  DIALOG_MOVE_PAGE,
  DIALOG_SORT_PAGES,
} from '@/lib/registries'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { FilePlus, FolderPlus, List, MoreVertical, Move, Repeat2, Trash } from 'lucide-react'
import { useCallback } from 'react'
import { useLocation } from 'react-router'
import { toast } from 'sonner'
import { TreeViewActionButton } from './TreeViewActionButton'

export type MoreActionsProps = {
  node: PageNode
}

export default function MoreActionsMenu({ node }: MoreActionsProps) {
  const { id: nodeId, kind: nodeKind, children } = node
  const openDialog = useDialogsStore((state) => state.openDialog)
  const reloadTree = useTreeStore((state) => state.reloadTree)
  const hasChildren = children && children.length > 0
  const location = useLocation()

  const handleConvertPage = useCallback(() => {
    convertPage(nodeId, nodeKind === NODE_KIND_PAGE ? NODE_KIND_SECTION : NODE_KIND_PAGE).then(() => {
      toast.success('Page converted successfully')
      reloadTree()
    }).catch(() => {
      toast.error('Failed to convert page')
    })
  }, [nodeId, nodeKind, reloadTree])

  const redirectUrlAfterDelete = useCallback(() => {
    if (location.pathname.startsWith("/" + node.path)) {
      if (node.parentId) {
        return node.path.substring(0, node.path.lastIndexOf('/'))
      } else {
        return '/'
      }
    }

    // remove leading slash
    return location.pathname.startsWith("/") ? location.pathname.substring(1) : location.pathname
  }, [location.pathname, node.path, node.parentId])

  return (
    <DropdownMenu>
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
        {nodeKind === NODE_KIND_SECTION && hasChildren && (
          <DropdownMenuItem
            className="cursor-pointer"
            onClick={() => openDialog(DIALOG_SORT_PAGES, { parent: node })}
          >
            <List size={18} className="tree-node__action-icon" /> Sort Section
          </DropdownMenuItem>
        )}
        <DropdownMenuItem
          className="cursor-pointer"
          onClick={() => openDialog(DIALOG_MOVE_PAGE, { pageId: node.id })}
        >
          <Move size={18} className="tree-node__action-icon" /> Move{' '}
          {nodeKind === NODE_KIND_PAGE ? 'page' : 'section'}
        </DropdownMenuItem>
        {nodeKind === NODE_KIND_SECTION && !hasChildren && (
          <DropdownMenuItem className="cursor-pointer" onClick={handleConvertPage}><Repeat2 size={18} className="tree-node__action-icon" />
            {' '}
            Convert to page
          </DropdownMenuItem>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem className="cursor-pointer text-error" onClick={() => {
          openDialog(DIALOG_DELETE_PAGE_CONFIRMATION, {
            pageId: node?.id,
            redirectUrl: redirectUrlAfterDelete(),
          })
        }}><Trash size={18} className="tree-node__action-icon text-error" /> Delete{' '}
          {nodeKind === NODE_KIND_PAGE ? 'page' : 'section'}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
