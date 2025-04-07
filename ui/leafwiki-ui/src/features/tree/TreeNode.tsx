import { useTreeStore } from '@/stores/tree';
import { ChevronUp, File } from 'lucide-react';
import React, { useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { PageNode } from '../../lib/api';
import { AddPageDialog } from '../page/AddPageDialog';
import { MovePageButton } from '../page/MovePageButton';
import { SortPagesDialog } from '../page/SortPagesDialog';

type Props = {
  node: PageNode
  level?: number
  onChildOpen?: (height: number) => void
}

export function TreeNode({ node, level = 0, onChildOpen  }: Props) {
  const { isNodeOpen, toggleNode, searchQuery } = useTreeStore()
  const hasChildren = node.children && node.children.length > 0
  const [hovered, setHovered] = useState(false)
  const { pathname } = useLocation()
  const isActive = `/${node.path}` === pathname
  const open = isNodeOpen(node.id)

  const handleChildOpen = (height: number) => {
    console.log('child open', height)
    if (contentRef.current) {
      const refHeight = contentRef.current.scrollHeight
      const calcHeight = (open?height+refHeight:0)
      setMaxHeight(calcHeight)
      if (onChildOpen) {
        onChildOpen(calcHeight)
      }
    }
  }

  useEffect(() => {
    if (open) {
      const refHeight = contentRef.current?.scrollHeight || 0
      setMaxHeight(refHeight)
      if (onChildOpen) {
        onChildOpen(refHeight)
      }
    } else {
      setMaxHeight(0)
    }
  }, [open, node.id, onChildOpen])

  const [maxHeight, setMaxHeight] = useState<number>(0);
  const contentRef = useRef<HTMLDivElement>(null);
  
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

  const linkText = (<Link to={`/${node.path}`}>
    <span className="block w-[130px] overflow-hidden truncate text-ellipsis">
      {highlightTitle()}
    </span>
  </Link>)


  return (
    <div>
      <div
        className={`flex cursor-pointer items-center text-base transition-all ease-in-out duration-200 rounded-lg ${isActive ? 'bg-gray-200 font-semibold' : 'hover:bg-gray-100 text-gray-800'
          }`}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      >
        {hasChildren && (
          <>
            <div className='pt-1 pb-1 pr-1 flex flex-grow items-center gap-2'>
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  toggleNode(node.id)
                }}
                className={`text-gray-600 hover:text-gray-800 transition-all ease-in-out ${hasChildren
                  ? 'opacity-100'
                  : 'opacity-0 group-hover:opacity-100'
                  }`}
              >
                <ChevronUp size={18}
                  className={`${open ? 'rotate-180' : 'rotate-90'} transition-all ease-in-out`}
                />
              </button>
              {linkText}
            </div>
          </>
        )}


        {!hasChildren && <div className="pt-1 pb-1 pr-1 flex flex-grow items-center gap-2"><File size={18} className="text-gray-400" />{linkText}</div>}

        {hovered && (
          <div className="flex flex-shrink-0 items-center gap-1">
            <AddPageDialog parentId={node.id} minimal />
            <MovePageButton pageId={node.id} />
            {hasChildren && <SortPagesDialog parent={node} />}
          </div>
        )}
      </div>
      <div
        ref={contentRef}
        className="ml-4 space-y-1 transition-all duration-300 ease-in-out"
        style={{
          maxHeight: `${maxHeight}px`,
          opacity: open ? 1 : 0,
//          transition: 'max-height 0.3s ease-out, opacity 0.3s ease-out',
          overflow: 'hidden',
        }}
      >
        {hasChildren &&
          node.children.map((child) => (
            <React.Fragment key={child.id}>
              <TreeNode node={child} level={level + 1} onChildOpen={handleChildOpen} />
            </React.Fragment>
          ))}
      </div>
    </div>
  )
}
