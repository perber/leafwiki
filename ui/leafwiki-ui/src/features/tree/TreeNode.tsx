import { ChevronDown, ChevronRight, FileText } from "lucide-react"
import { useState } from "react"
import { PageNode } from "../../lib/api"

type Props = {
  node: PageNode
  level?: number
  onSelect?: (id: string) => void
}

export function TreeNode({ node, level = 0, onSelect }: Props) {
  const [open, setOpen] = useState(true)
  const hasChildren = node.children && node.children.length > 0

  return (
    <div className="pl-2">
      <div
        className="flex items-center gap-1 cursor-pointer text-sm text-gray-800 hover:underline"
        onClick={() => onSelect?.(node.id)}
      >
        {hasChildren && (
          <button
            onClick={e => {
              e.stopPropagation()
              setOpen(prev => !prev)
            }}
            className="p-0.5 text-gray-500 hover:text-gray-800"
          >
            {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          </button>
        )}

        {!hasChildren && <FileText size={14} className="text-gray-400" />}
        <span>{node.title}</span>
      </div>

      {open && hasChildren && (
        <div className="ml-4 space-y-1">
          {node.children.map(child => (
            <TreeNode key={child.id} node={child} level={level + 1} onSelect={onSelect} />
          ))}
        </div>
      )}
    </div>
  )
}
