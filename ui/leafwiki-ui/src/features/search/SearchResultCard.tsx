import i18next from '@/lib/i18n'
import { SearchResultItem } from '@/lib/api/search'

const t = (key: string, opts?: object) =>
  i18next.t(key, { ...opts, ns: 'search' })
import { createNavigationVisitState } from '@/lib/navigationVisit'
import { buildViewUrl } from '@/lib/routePath'
import { normalizeWikiRoutePath } from '@/lib/wikiPath'
import { forwardRef } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { usePageEditorStore } from '../editor/pageEditorStore'
import HighlightedSearchTitle from './HighlightedSearchTitle'

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
    const currentEditorPageId = usePageEditorStore(
      (state) => state.page?.id ?? state.initialPage?.id,
    )
    const currentViewPath = normalizeWikiRoutePath(
      buildViewUrl(location.pathname),
    )
    const resultPath = normalizeWikiRoutePath(item.path)
    const resultUrl = `${resultPath}${location.search}`
    const isRouteActive = currentViewPath === resultPath
    const isEditorActive = currentEditorPageId === item.page_id
    const isActive = isRouteActive || isEditorActive || isSelected
    const kindLabel =
      item.kind === 'section'
        ? t('resultCard.kindSection')
        : t('resultCard.kindPage')

    return (
      <Link
        ref={ref}
        to={resultUrl}
        state={createNavigationVisitState()}
        data-testid={`search-result-card-${item.page_id}`}
        aria-current={isRouteActive ? 'page' : undefined}
        onMouseEnter={onMouseEnter}
        onFocus={onFocus}
        className={`list-view__item search-result-card ${
          isActive ? 'list-view__item--active search-result-card--selected' : ''
        } ${isRouteActive ? 'search-result-card--route-active' : ''}`.trim()}
      >
        <div
          className="search-result-card__title"
          data-testid={`search-result-card-title-${item.page_id}`}
        >
          <HighlightedSearchTitle text={item.title} />
        </div>
        <div className="search-result-card__excerpt">
          <HighlightedSearchTitle text={item.excerpt} />
        </div>
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
