import { SearchResultItem } from '@/lib/api/search'
import { Link, useLocation } from 'react-router-dom'

export default function SearchResultCard({ item }: { item: SearchResultItem }) {
  const location = useLocation()
  const isActive = location.pathname === `${item.path}`
  return (
    <Link
      to={`${item.path}`}
      data-testid={`search-result-card-${item.page_id}`}
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
}
