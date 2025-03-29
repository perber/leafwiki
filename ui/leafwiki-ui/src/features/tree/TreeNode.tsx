import { useTreeStore } from "@/stores/tree"
import { ChevronDown, ChevronRight, FileText } from "lucide-react"
import React, { useState } from "react"
import { Link, useLocation } from "react-router-dom"
import { PageNode } from "../../lib/api"
import { TreeAddInline } from "./TreeAddInline"

type Props = {
  node: PageNode
  level?: number
}

export function TreeNode({ node, level = 0 }: Props) {
  const { isNodeOpen, toggleNode } = useTreeStore()
  const hasChildren = node.children && node.children.length > 0
  const [hovered, setHovered] = useState(false)
  const { pathname } = useLocation()
  const isActive = `/${node.path}` === pathname
  const open = isNodeOpen(node.id)

  return (
    <div className="pl-2 relative">
      <div
        className={`flex items-center gap-1 cursor-pointer text-sm hover:underline ${isActive ? "bg-gray-200 font-semibold" : "hover:bg-gray-100"}`}
        onMouseEnter={() => setHovered(true)} onMouseLeave={() => setHovered(false)}
      >
        {hasChildren && (
          <button
            onClick={e => {
              e.stopPropagation()
              toggleNode(node.id)
            }}
            className={`p-0.5 text-gray-500 hover:text-gray-800 transition-opacity ${
              hasChildren ? "opacity-100" : "opacity-0 group-hover:opacity-100"
            }`}
          >
            {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          </button>
        )}

        {!hasChildren && <FileText size={14} className="text-gray-400" />}
        <Link to={`/${node.path}`}>
          <span>{node.title}</span>
        </Link>

        {hovered && (
          <div className="absolute right-1 top-0">
            <TreeAddInline parentId={node.id} minimal />
          </div>
        )}

      </div>

      {open && (
        <div className="ml-4 space-y-1">
          {hasChildren &&
            node.children.map(child => (
              <React.Fragment key={child.id}>
                <TreeNode node={child} level={level + 1} />
              </React.Fragment>
            ))}
        </div>
      )}
    </div>
  )
}
