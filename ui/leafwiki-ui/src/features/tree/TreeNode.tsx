import { TooltipWrapper } from '@/components/TooltipWrapper'
import { TreeViewActionButton } from '@/features/tree/TreeViewActionButton'
import { PageNode } from '@/lib/api/pages'
import {
  DIALOG_ADD_PAGE,
  DIALOG_MOVE_PAGE,
  DIALOG_SORT_PAGES,
} from '@/lib/registries'
import { buildEditUrl, buildViewUrl } from '@/lib/urlUtil'
import { useAppMode } from '@/lib/useAppMode'
import { useIsMobile } from '@/lib/useIsMobile'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import clsx from 'clsx'
import { ChevronUp, List, Move, Plus } from 'lucide-react'
import React, { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'

type Props = {
  node: PageNode
  level?: number
}

export const TreeNode = React.memo(function TreeNode({
  node,
  level = 0,
}: Props) {
  const { isNodeOpen, toggleNode } = useTreeStore()
  const appMode = useAppMode()
  const hasChildren = node.children && node.children.length > 0
  const [hovered, setHovered] = useState(false)
  const { pathname } = useLocation()

  const currentPath =
    appMode === 'edit'
      ? buildEditUrl(node.path)
      : buildViewUrl(node.path.startsWith('/') ? node.path : `/${node.path}`)

  const isActive = currentPath === pathname
  const open = isNodeOpen(node.id)
  const openDialog = useDialogsStore((state) => state.openDialog)

  const isMobile = useIsMobile()
  const readOnlyMode = useIsReadOnly()

  const indent = 4
  const markerOffset = 8 // Distance from left for the vertical line

  const linkText = (
    <TooltipWrapper
      label={node.title}
      side="bottom"
      align="center"
      parentClassName="tree-node__tooltip-parent"
    >
      <Link
        to={`/${node.path}`}
        className="tree-node__link"
        data-testid={`tree-node-link-${node.id}`}
      >
        <span
          className={clsx('tree-node__title', {
            'tree-node__title--active': isActive,
          })}
        >
          {node.title || 'Untitled Page'}
        </span>
      </Link>
    </TooltipWrapper>
  )

  const treeActionButtonStyle = isMobile ? '' : 'tree-node__actions--compact'

  return (
    <>
      <div
        className={clsx('tree-node', {
          'tree-node--active': isActive,
          'tree-node--inactive': !isActive,
        })}
        data-testid={`tree-node-${node.id}`}
        style={{ paddingLeft: indent }}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        <div
          className={clsx('tree-node__marker', {
            'tree-node__marker--active': isActive,
          })}
          style={{ left: markerOffset }}
        />

        <div className="tree-node__main">
          {hasChildren && (
            <ChevronUp
              data-testid={`tree-node-toggle-icon-${node.id}`}
              size={16}
              className={clsx('tree-node__toggle', {
                'tree-node__toggle--open': open,
                'tree-node__toggle--closed': !open,
              })}
              onClick={() => hasChildren && toggleNode(node.id)}
            />
          )}
          {
            // add empty space to align with nodes that have children
            !hasChildren && <div className="tree-node__toggle-spacer" />
          }
          {linkText}
        </div>

        {(hovered || isMobile) && !readOnlyMode && (
          <div className={clsx('tree-node__actions', treeActionButtonStyle)}>
            <TreeViewActionButton
              actionName="add"
              icon={<Plus size={18} className="tree-node__action-icon" />}
              tooltip="Create new page"
              onClick={() => openDialog(DIALOG_ADD_PAGE, { parentId: node.id })}
            />
            <TreeViewActionButton
              actionName="move"
              icon={<Move size={16} className="tree-node__action-icon" />}
              tooltip="Move page to new parent"
              onClick={() => openDialog(DIALOG_MOVE_PAGE, { pageId: node.id })}
            />
            {hasChildren && (
              <TreeViewActionButton
                actionName="sort"
                icon={<List size={16} className="tree-node__action-icon" />}
                tooltip="Sort pages"
                onClick={() => openDialog(DIALOG_SORT_PAGES, { parent: node })}
              />
            )}
          </div>
        )}
      </div>

      <div
        className={clsx('tree-node__children', {
          'tree-node__children--closed': !open,
        })}
      >
        {hasChildren &&
          node.children?.map((child) => (
            <TreeNode key={child.id} node={child} level={level + 1} />
          ))}
      </div>
    </>
  )
})
