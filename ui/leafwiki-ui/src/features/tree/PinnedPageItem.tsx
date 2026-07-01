import { NODE_KIND_SECTION, PageNode } from '@/lib/api/pages'
import { buildViewUrl } from '@/lib/routePath'
import { useTreeStore } from '@/stores/tree'
import clsx from 'clsx'
import { File, Folder, PinOff } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router-dom'

type Props = {
  node: PageNode
  onUnpin: () => void
}

export function PinnedPageItem({ node, onUnpin }: Props) {
  const activeNodeId = useTreeStore((s) => s.activeNodeId)
  const isActive = activeNodeId === node.id
  const [hovered, setHovered] = useState(false)
  const Icon = node.kind === NODE_KIND_SECTION ? Folder : File

  return (
    <div
      className={clsx('tree-view__pinned-item', {
        'tree-view__pinned-item--active': isActive,
      })}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      data-testid="pinned-page-item"
    >
      <Link
        to={buildViewUrl(`/${node.path}`)}
        className="tree-view__pinned-item-link"
      >
        <Icon size={13} className="tree-view__pinned-item-icon" />
        <span className="tree-view__pinned-item-title">{node.title}</span>
      </Link>
      {hovered && (
        <button
          className="tree-view__pinned-item-unpin"
          onClick={(e) => {
            e.preventDefault()
            onUnpin()
          }}
          title="Unpin"
          aria-label="Unpin page"
        >
          <PinOff size={13} />
        </button>
      )}
    </div>
  )
}
