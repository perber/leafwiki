import { SearchResultItem } from '@/lib/api/search'
import { forwardRef } from 'react'
import { Link, useLocation } from 'react-router-dom'

type SearchResultCardProps = {
  item: SearchResultItem
  isSelected?: boolean
  onMouseEnter?: () => void
  onFocus?: () => void
}

const SearchResultCard = forwardRef<HTMLAnchorElement, SearchResultCardProps>(
  function SearchResultCard(
    { item, isSelected = false, onMouseEnter, onFocus },
    ref,
  ) {
    const location = useLocation()
    const isRouteActive = location.pathname === `${item.path}`
    const kindLabel = item.kind === 'section' ? 'Section' : 'Page'

    return (
      <Link
        ref={ref}
        to={`${item.path}`}
        data-testid={`search-result-card-${item.page_id}`}
        aria-current={isRouteActive ? 'page' : undefined}
        onMouseEnter={onMouseEnter}
        onFocus={onFocus}
        className={`list-view__item search-result-card ${
          isSelected
            ? 'list-view__item--active search-result-card--selected'
            : ''
        } ${isRouteActive ? 'search-result-card--route-active' : ''}`.trim()}
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
        <div className="search-result-card__meta">
          <span className="search-result-card__badge">{kindLabel}</span>
        </div>
        <div className="search-result-card__path">
          {item.path.split('/').join(' / ')}
        </div>
      </Link>
    )
  },
)

export default SearchResultCard
