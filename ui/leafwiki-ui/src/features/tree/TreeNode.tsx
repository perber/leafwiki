import { TreeViewActionButton } from '@/components/TreeViewActionButton'
import { useMeasure } from '@/lib/useMeasure'
import { useDialogsStore } from '@/stores/dialogs'
import { useTreeStore } from '@/stores/tree'
import { ChevronUp, File, Folder, List, Move, Plus } from 'lucide-react'
import { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { PageNode } from '../../lib/api'

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
  const openDialog = useDialogsStore((state) => state.openDialog)

  const [ref] = useMeasure<HTMLDivElement>()

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

  const linkText = (
    <Link to={`/${node.path}`}>
      <span className="block w-[150px] overflow-hidden truncate text-ellipsis">
        {highlightTitle()}
      </span>
    </Link>
  )

  return (
    <div>
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
          {hasChildren && (<Folder size={18} className="text-gray-400" />)}

          {linkText}
        </div>

        {hovered && (
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
                  size={20}
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
    </div>
  )
}
