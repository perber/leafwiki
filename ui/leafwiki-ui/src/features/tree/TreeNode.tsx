import { TreeViewActionButton } from '@/features/tree/TreeViewActionButton'
import { NODE_KIND_SECTION, PageNode } from '@/lib/api/pages'
import { DIALOG_ADD_PAGE } from '@/lib/registries'
import { createNavigationVisitState } from '@/lib/navigationVisit'
import { useIsMobile } from '@/lib/useIsMobile'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { useDraggable, useDroppable } from '@dnd-kit/core'
import clsx from 'clsx'
import { ChevronUp, FilePlus, FolderPlus } from 'lucide-react'
import React, { useState } from 'react'
import { Link } from 'react-router-dom'
import { useTreeDnd } from './treeDndContext'
import { useTreeNodeActionsMenusStore } from './treeNodeActionsMenus'
import TreeNodeActionsMenu from './TreeNodeActionsMenu'

type Props = {
  node: PageNode
}

export const TreeNode = React.memo(function TreeNode({ node }: Props) {
  const open = useTreeStore((s) => !!s.openNodeIdSet?.[node.id])
  const isStoreActive = useTreeStore((s) => s.activeNodeId === node.id)
  const toggleNode = useTreeStore((s) => s.toggleNode)
  const hasChildren = node.children && node.children.length > 0
  const openDialog = useDialogsStore((state) => state.openDialog)
  const isMobile = useIsMobile()
  const readOnlyMode = useIsReadOnly()
  const [hovered, setHovered] = useState(false)
  const isActionsMenuOpen = useTreeNodeActionsMenusStore(
    (s) => s.openMenuNodeId === node.id,
  )
  const isActive = isStoreActive

  const dnd = useTreeDnd()
  const {
    setNodeRef: setDragRef,
    listeners,
    isDragging,
  } = useDraggable({
    id: node.id,
    data: { node },
    disabled: !dnd.enabled,
  })
  const { setNodeRef: setDropRef } = useDroppable({
    id: node.id,
    data: { node },
    disabled: !dnd.enabled,
  })
  const setRowRef = (el: HTMLElement | null) => {
    setDragRef(el)
    setDropRef(el)
  }
  const dropTarget = dnd.dropTarget?.nodeId === node.id ? dnd.dropTarget : null

  const indent = 4
  const markerOffset = 8 // Distance from left for the vertical line

  const linkText = (
    <div className={clsx('flex', 'tree-node__tooltip-parent')}>
      <Link
        to={`/${node.path}`}
        state={createNavigationVisitState()}
        className="tree-node__link"
        data-testid={`tree-node-link-${node.id}`}
        aria-current={isActive ? 'page' : undefined}
        draggable={false}
      >
        <span
          className={clsx('tree-node__title', {
            'tree-node__title--active': isActive,
          })}
        >
          {node.title || 'Untitled Page'}
        </span>
      </Link>
    </div>
  )

  const treeActionButtonStyle = isMobile ? '' : 'tree-node__actions--compact'

  return (
    <>
      <div
        ref={setRowRef}
        {...(dnd.enabled ? listeners : {})}
        className={clsx('tree-node', {
          'tree-node--active': isActive,
          'tree-node--inactive': !isActive,
          'tree-node--dragging': isDragging,
          'tree-node--drop-inside': dropTarget?.zone === 'inside',
        })}
        data-testid={`tree-node-${node.id}`}
        style={{ paddingLeft: indent }}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        {dropTarget?.zone === 'before' && (
          <div className="tree-node__drop-line tree-node__drop-line--top" />
        )}
        {dropTarget?.zone === 'after' && (
          <div className="tree-node__drop-line tree-node__drop-line--bottom" />
        )}
        {dropTarget?.zone === 'inside' && node.kind !== NODE_KIND_SECTION && (
          // Nesting into a page converts it into a section on drop
          <FolderPlus size={14} className="tree-node__nest-hint" />
        )}
        <div
          className={clsx('tree-node__marker', {
            'tree-node__marker--active': isActive,
          })}
          style={{ left: markerOffset }}
        />

        <div className="tree-node__main">
          {node.kind === NODE_KIND_SECTION && (
            <ChevronUp
              data-testid={`tree-node-toggle-icon-${node.id}`}
              size={16}
              className={clsx('tree-node__toggle', {
                'tree-node__toggle--open': open,
                'tree-node__toggle--closed': !open,
              })}
              onClick={() =>
                node.kind === NODE_KIND_SECTION && toggleNode(node.id)
              }
            />
          )}
          {
            // add empty space to align with nodes that have children
            node.kind !== NODE_KIND_SECTION && (
              <div className="tree-node__toggle-spacer" />
            )
          }
          {linkText}
          {!readOnlyMode && (isMobile || hovered || isActionsMenuOpen) && (
            <div className={clsx('tree-node__actions', treeActionButtonStyle)}>
              <TreeViewActionButton
                actionName="add"
                icon={
                  <FilePlus
                    size={18}
                    className={clsx(
                      'tree-node__action-icon',
                      isMobile && 'text-brand/70!',
                    )}
                  />
                }
                tooltip="Create new page"
                onClick={() =>
                  openDialog(DIALOG_ADD_PAGE, { parentId: node.id })
                }
              />
              <TreeNodeActionsMenu node={node} />
            </div>
          )}
        </div>
      </div>

      <div
        className={clsx('tree-node__children', {
          'tree-node__children--closed': !open,
          'tree-node__children--dragging': dnd.activeId === node.id,
        })}
      >
        {hasChildren &&
          node.children?.map((child) => (
            <TreeNode key={child.id} node={child} />
          ))}
      </div>
    </>
  )
})
