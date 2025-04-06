import { useTreeStore } from '@/stores/tree'
import { ChevronDown, ChevronRight, FileText } from 'lucide-react'
import React, { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { PageNode } from '../../lib/api'
import { AddPageDialog } from '../page/AddPageDialog'
import { MovePageButton } from '../page/MovePageButton'
import { SortPagesButton } from '../page/SortPagesButton'

type Props = {
  node: PageNode
  level?: number
}

export function TreeNode({ node, level = 0 }: Props) {
  const { isNodeOpen, toggleNode, searchQuery } = useTreeStore()
  const hasChildren = node.children && node.children.length > 0
  const [hovered, setHovered] = useState(false)
  const { pathname } = useLocation()
  const isActive = `/${node.path}` === pathname
  const open = isNodeOpen(node.id)

  const highlightTitle = () => {
    if (!searchQuery) return node.title

    const index = node.title.toLowerCase().indexOf(searchQuery.toLowerCase())
    if (index === -1) return node.title

    const before = node.title.slice(0, index)
    const match = node.title.slice(index, index + searchQuery.length)
    const after = node.title.slice(index + searchQuery.length)

    return (
      <>
        {before}
        <mark className="bg-yellow-200 text-black">{match}</mark>
        {after}
      </>
    )
  }

  return (
    <div className="pl-2">
      <div
        className={`flex cursor-pointer items-center gap-1 text-sm hover:underline ${isActive ? 'bg-gray-200 font-semibold' : 'hover:bg-gray-100'}`}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        <div className="flex flex-grow items-center gap-1">
          {hasChildren && (
            <button
              onClick={(e) => {
                e.stopPropagation()
                toggleNode(node.id)
              }}
              className={`p-0.5 text-gray-500 transition-opacity hover:text-gray-800 ${
                hasChildren
                  ? 'opacity-100'
                  : 'opacity-0 group-hover:opacity-100'
              }`}
            >
              {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
            </button>
          )}
          {!hasChildren && <FileText size={14} className="text-gray-400" />}
          <Link to={`/${node.path}`}>
            <span className="block w-[130px] overflow-hidden truncate text-ellipsis">
              {highlightTitle()}
            </span>
          </Link>
        </div>
        {hovered && (
          <div className="flex flex-shrink-0 items-center gap-1">
            <AddPageDialog parentId={node.id} minimal />
            <MovePageButton pageId={node.id} />
            {hasChildren && <SortPagesButton parent={node} />}
          </div>
        )}
      </div>

      {open && (
        <div className="ml-4 space-y-1">
          {hasChildren &&
            node.children.map((child) => (
              <React.Fragment key={child.id}>
                <TreeNode node={child} level={level + 1} />
              </React.Fragment>
            ))}
        </div>
      )}
    </div>
  )
}
