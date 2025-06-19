import { TooltipWrapper } from '@/components/TooltipWrapper'
import { TreeViewActionButton } from '@/components/TreeViewActionButton'
import { useIsMobile } from '@/lib/useIsMobile'
import { useIsReadOnly } from '@/lib/useIsReadOnly'
import { useMeasure } from '@/lib/useMeasure'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { ChevronUp, File, Folder, List, Move, Plus } from 'lucide-react'
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

  const linkText = (
    <TooltipWrapper label={node.title} side="top" align="start">
      <Link to={`/${node.path}`}>
        <span className="block w-[150px] overflow-hidden truncate text-ellipsis">
          {node.title || 'Untitled Page'}
        </span>
      </Link>
    </TooltipWrapper>
  )

  return (
    <>
      <div
        className={`flex cursor-pointer items-center rounded-lg pb-1 pt-1 text-base transition-all duration-200 ease-in-out ${
          isActive
            ? 'bg-gray-200 font-semibold'
            : 'text-gray-800 hover:bg-gray-200'
        }`}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        <div className="flex flex-1 items-center gap-2">
          {hasChildren && (
            <ChevronUp
              size={16}
              className={`transition-transform ${open ? 'rotate-180' : 'rotate-90'}`}
              onClick={() => hasChildren && toggleNode(node.id)}
            />
          )}

          {/* Zeigt das File-Icon für Knoten ohne Kinder */}
          {!hasChildren && <File size={18} className="text-gray-400" />}
          {hasChildren && <Folder size={18} className="text-gray-400" />}

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

      <div
        ref={ref}
        className={`ml-4 pl-2 ease-in-out ${!open ? 'overflow-hidden' : ''}`}
        style={{
          maxHeight: open ? `1000px` : '0px',
          opacity: open ? 1 : 0,
        }}
      >
        {hasChildren &&
          node.children.map((child) => (
            <TreeNode key={child.id} node={child} level={level + 1} />
          ))}
      </div>
    </>
  )
})
