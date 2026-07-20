import { PageNode } from '@/lib/api/pages'
import { useFavoritesStore } from '@/stores/favorites'
import { useTreeStore } from '@/stores/tree'
import { FavoriteItem } from './FavoriteItem'

export function FavoritesSection() {
  const favoritePageIds = useFavoritesStore((s) => s.favoritePageIds)
  const byId = useTreeStore((s) => s.byId)

  const favoritePages = Array.from(favoritePageIds)
    .map((id) => byId[id])
    .filter((n): n is PageNode => !!n)
    .sort((a, b) => a.title.localeCompare(b.title))

  if (favoritePages.length === 0) return null

  return (
    <div className="tree-view__favorites" data-testid="favorites-section">
      {favoritePages.map((node) => (
        <FavoriteItem key={node.id} node={node} />
      ))}
    </div>
  )
}
