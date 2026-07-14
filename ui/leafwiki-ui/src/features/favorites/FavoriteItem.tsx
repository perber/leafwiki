import { FavoriteToggleButton } from '@/features/favorites/FavoriteToggleButton'
import { NODE_KIND_SECTION, PageNode } from '@/lib/api/pages'
import { buildViewUrl } from '@/lib/routePath'
import { useTreeStore } from '@/stores/tree'
import clsx from 'clsx'
import { File, Folder } from 'lucide-react'
import { Link } from 'react-router-dom'

type Props = {
  node: PageNode
}

export function FavoriteItem({ node }: Props) {
  const activeNodeId = useTreeStore((s) => s.activeNodeId)
  const isActive = activeNodeId === node.id
  const Icon = node.kind === NODE_KIND_SECTION ? Folder : File

  return (
    <div
      className={clsx('tree-view__favorite-item', {
        'tree-view__favorite-item--active': isActive,
      })}
      data-testid="favorite-item"
    >
      <Link
        to={buildViewUrl(`/${node.path}`)}
        className="tree-view__favorite-item-link"
      >
        <Icon size={13} className="tree-view__favorite-item-icon" />
        <span className="tree-view__favorite-item-title">{node.title}</span>
      </Link>
      <FavoriteToggleButton
        pageId={node.id}
        className="tree-view__favorite-item-remove"
      />
    </div>
  )
}
