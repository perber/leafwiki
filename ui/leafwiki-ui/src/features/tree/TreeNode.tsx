import { TooltipWrapper } from '@/components/TooltipWrapper'
import { TreeViewActionButton } from '@/features/tree/TreeViewActionButton'
import { NODE_KIND_SECTION, PageNode } from '@/lib/api/pages'
import { DIALOG_ADD_PAGE } from '@/lib/registries'
import { buildEditUrl, buildViewUrl } from '@/lib/urlUtil'
import { useAppMode } from '@/lib/useAppMode'
import { useIsMobile } from '@/lib/useIsMobile'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import clsx from 'clsx'
import { ChevronUp, FilePlus } from 'lucide-react'
import React from 'react'
import { Link, useLocation } from 'react-router-dom'
import TreeNodeActionsMenu from './TreeNodeActionsMenu'

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
      >
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
          {!readOnlyMode && (
            <div className={clsx('tree-node__actions', treeActionButtonStyle)}>
              <TreeViewActionButton
                actionName="add"
                icon={<FilePlus size={18} className="tree-node__action-icon" />}
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
