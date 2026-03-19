import { SearchResultItem } from '@/lib/api/search'
import { forwardRef } from 'react'
import { Link, useLocation } from 'react-router-dom'

type SearchResultCardProps = {
  item: SearchResultItem
  isSelected?: boolean
}

const SearchResultCard = forwardRef<HTMLAnchorElement, SearchResultCardProps>(
  function SearchResultCard({ item, isSelected = false }, ref) {
    const location = useLocation()
    const isRouteActive = location.pathname === `${item.path}`
    const isActive = isRouteActive || isSelected

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
