import { TooltipWrapper } from '@/components/TooltipWrapper'
import { useFavoritesStore } from '@/stores/favorites'
import clsx from 'clsx'
import { Star } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

type Props = {
  pageId: string
  size?: number
  className?: string
}

export function FavoriteToggleButton({ pageId, size = 13, className }: Props) {
  const { t } = useTranslation('viewer')
  const isFavorited = useFavoritesStore((s) => s.favoritePageIds.has(pageId))
  const addFavorite = useFavoritesStore((s) => s.addFavorite)
  const removeFavorite = useFavoritesStore((s) => s.removeFavorite)

  const handleToggle = async () => {
    try {
      if (isFavorited) {
        await removeFavorite(pageId)
        toast.success(t('favorites.removeSuccess'))
      } else {
        await addFavorite(pageId)
        toast.success(t('favorites.addSuccess'))
      }
    } catch {
      toast.error(t('favorites.favoriteError'))
    }
  }

  const label = isFavorited
    ? t('favorites.removeFavorite')
    : t('favorites.addFavorite')

  return (
    <TooltipWrapper label={label} side="top" align="start">
      <button
        type="button"
        className={clsx('favorite-toggle-button', className, {
          'favorite-toggle-button--active': isFavorited,
        })}
        onClick={(e) => {
          e.preventDefault()
          e.stopPropagation()
          handleToggle()
        }}
        aria-label={label}
        aria-pressed={isFavorited}
        data-testid={`favorite-toggle-${pageId}`}
      >
        <Star size={size} fill={isFavorited ? 'currentColor' : 'none'} />
      </button>
    </TooltipWrapper>
  )
}
