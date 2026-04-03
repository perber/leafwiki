import { SearchResultItem } from '@/lib/api/search'
import { buildViewUrl } from '@/lib/routePath'
import { normalizeWikiRoutePath } from '@/lib/wikiPath'
import { forwardRef } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { usePageEditorStore } from '../editor/pageEditor'

type SearchResultCardProps = {
  item: SearchResultItem
  isSelected?: boolean
}

const SearchResultCard = forwardRef<HTMLAnchorElement, SearchResultCardProps>(
  function SearchResultCard({ item, isSelected = false }, ref) {
    const location = useLocation()
    const currentEditorPageId = usePageEditorStore(
      (state) => state.page?.id ?? state.initialPage?.id,
    )
    const currentViewPath = normalizeWikiRoutePath(
      buildViewUrl(location.pathname),
    )
    const resultPath = normalizeWikiRoutePath(item.path)
    const isRouteActive = currentViewPath === resultPath
    const isEditorActive = currentEditorPageId === item.page_id
    const isActive = isRouteActive || isEditorActive || isSelected

    return (
      <Link
        ref={ref}
        to={`${item.path}`}
        data-testid={`search-result-card-${item.page_id}`}
        aria-current={isRouteActive ? 'page' : undefined}
        className={`search-result-card ${
          isActive
            ? 'search-result-card--active'
            : 'search-result-card--inactive'
        }`}
      >
        <div
          className="search-result-card__title"
          data-testid={`search-result-card-title-${item.page_id}`}
          dangerouslySetInnerHTML={{ __html: item.title }}
        />
        <div
          className="search-result-card__excerpt"
          dangerouslySetInnerHTML={{ __html: item.excerpt }}
        />
        <div className="search-result-card__path">
          {item.path.split('/').join(' / ')}
        </div>
      </Link>
    )
  },
)

export default SearchResultCard
