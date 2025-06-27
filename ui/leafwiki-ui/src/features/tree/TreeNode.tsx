import { TooltipWrapper } from '@/components/TooltipWrapper'
import { TreeViewActionButton } from '@/features/tree/TreeViewActionButton'
import { useIsMobile } from '@/lib/useIsMobile'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useMeasure } from '@/lib/useMeasure'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { ChevronUp, List, Move, Plus } from 'lucide-react'
import React, { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { PageNode } from '../../lib/api'

type Props = {
  node: PageNode
  level?: number
}

export const TreeNode = React.memo(function TreeNode({
  node,
  level = 0,
}: Props) {
  const { isNodeOpen, toggleNode } = useTreeStore()
  const hasChildren = node.children && node.children.length > 0
  const [hovered, setHovered] = useState(false)
  const { pathname } = useLocation()
  const isActive = `/${node.path}` === pathname
  const open = isNodeOpen(node.id)
  const openDialog = useDialogsStore((state) => state.openDialog)

  const isMobile = useIsMobile()
  const [ref] = useMeasure<HTMLDivElement>()
  const readOnlyMode = useIsReadOnly()

  const indent = level * 16
  const markerOffset = 8 // Distance from left for the vertical line

  const linkText = (
    <TooltipWrapper label={node.title} side="top" align="start">
      <Link to={`/${node.path}`}>
        <span
          className={`block max-w-[200px] overflow-hidden truncate text-ellipsis ${
            level === 0
              ? 'text-base font-semibold'
              : level === 1
                ? 'text-sm text-gray-800'
                : 'text-sm text-gray-500'
          }`}
        >
          {node.title || 'Untitled Page'}
        </span>
      </Link>
    </TooltipWrapper>
  )

  return (
    <>
      <div
        className={`relative flex cursor-pointer items-center pb-1 pt-1 transition-all duration-200 ease-in-out ${
          isActive
            ? 'font-semibold text-green-700'
            : 'text-gray-800 hover:bg-gray-100'
        }`}
        style={{ paddingLeft: indent }}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        <div
          className={`absolute bottom-0 top-0 w-[2px] ${
            isActive ? 'bg-green-600' : 'bg-gray-200'
          }`}
          style={{ left: markerOffset }}
        />

        <div className="flex flex-1 items-center gap-2 pl-4">
          {hasChildren && (
            <ChevronUp
              size={16}
              className={`transition-transform ${
                open ? 'rotate-180' : 'rotate-90'
              }`}
              onClick={() => hasChildren && toggleNode(node.id)}
            />
          )}
          {linkText}
        </div>

        {(hovered || isMobile) && !readOnlyMode && (
          <div className="flex gap-0">
            <TreeViewActionButton
              icon={
                <Plus
                  size={20}
                  className="cursor-pointer text-gray-500 hover:text-gray-800"
                />
              }
              tooltip="Create new page"
              onClick={() => openDialog('add', { parentId: node.id })}
            />
            <TreeViewActionButton
              icon={
                <Move
                  size={16}
                  className="cursor-pointer text-gray-500 hover:text-gray-800"
                />
              }
              tooltip="Move page to new parent"
              onClick={() => openDialog('move', { pageId: node.id })}
            />
            {hasChildren && (
              <TreeViewActionButton
                icon={
                  <List
                    size={20}
                    className="cursor-pointer text-gray-500 hover:text-gray-800"
                  />
                }
                tooltip="Sort pages"
                onClick={() => openDialog('sort', { parent: node })}
              />
            )}
          </div>
        )}
      </div>

      <div ref={ref} className={`ml-4 pl-2 ${!open ? 'hidden' : ''}`}>
        {hasChildren &&
          node.children.map((child) => (
            <TreeNode key={child.id} node={child} level={level + 1} />
          ))}
      </div>
    </>
  )
})
