import { TaggedPage } from '@/lib/api/tags'
import { createNavigationVisitState } from '@/lib/navigationVisit'
import { buildViewUrl } from '@/lib/routePath'
import { normalizeWikiRoutePath } from '@/lib/wikiPath'
import { MouseEvent, forwardRef } from 'react'
import { Link, useLocation } from 'react-router-dom'
import type { PageEditorState } from '../editor/pageEditorStore'
import { usePageEditorStore } from '../editor/pageEditorStore'

type TagsResultCardProps = {
  item: TaggedPage
  activeTags: string[]
  isSelected?: boolean
  onMouseEnter?: () => void
  onFocus?: () => void
  onTagClick: (tag: string) => void
}

const MAX_VISIBLE_TAGS = 4

const TagsResultCard = forwardRef<HTMLDivElement, TagsResultCardProps>(
  function TagsResultCard(
    { item, activeTags, isSelected = false, onMouseEnter, onFocus, onTagClick },
    ref,
  ) {
    const location = useLocation()
    const currentEditorPageId = usePageEditorStore(
      (state: PageEditorState) => state.page?.id ?? state.initialPage?.id,
    )
    const currentViewPath = normalizeWikiRoutePath(
      buildViewUrl(location.pathname),
    )
    const resultPath = normalizeWikiRoutePath(`/${item.path}`)
    const isRouteActive = currentViewPath === resultPath
    const isEditorActive = currentEditorPageId === item.id
    const isActive = isRouteActive || isEditorActive || isSelected
    const displayPath = resultPath.split('/').join(' / ')
    const visibleTags = item.tags.slice(0, MAX_VISIBLE_TAGS)
    const hiddenTagCount = Math.max(item.tags.length - visibleTags.length, 0)

    const handleTagClick = (
      event: MouseEvent<HTMLButtonElement>,
      tag: string,
    ) => {
      event.preventDefault()
      event.stopPropagation()
      onTagClick(tag)
    }

    return (
      <div
        ref={ref}
        data-testid={`tags-result-${item.id}`}
        onMouseEnter={onMouseEnter}
        onFocus={onFocus}
        className={`list-view__item search-result-card ${
          isActive ? 'list-view__item--active search-result-card--selected' : ''
        } ${isRouteActive ? 'search-result-card--route-active' : ''}`.trim()}
      >
        <Link
          to={`/${item.path}`}
          state={createNavigationVisitState()}
          aria-current={isRouteActive ? 'page' : undefined}
          className="tags-result-card__link"
        >
          <div
            className="search-result-card__title browse-results__item-title"
            data-testid={`tags-result-card-title-${item.id}`}
          >
            {item.title}
          </div>
          <div className="search-result-card__meta">
            <span className="search-result-card__badge">Page</span>
          </div>
          {item.excerpt && (
            <div className="search-result-card__excerpt">{item.excerpt}</div>
          )}
          <div className="search-result-card__path">{displayPath}</div>
        </Link>
        <div className="tags-result-card__tags">
          {visibleTags.map((tag) => {
            const isTagActive = activeTags.includes(tag)
            return (
              <button
                key={tag}
                type="button"
                className={`tags-result-card__tag ${
                  isTagActive ? 'tags-result-card__tag--active' : ''
                }`.trim()}
                onClick={(event) => handleTagClick(event, tag)}
                aria-pressed={isTagActive}
                data-testid={`tags-result-tag-${item.id}-${tag}`}
              >
                {tag}
              </button>
            )
          })}
          {hiddenTagCount > 0 && (
            <span className="tags-result-card__tag-more">
              +{hiddenTagCount}
            </span>
          )}
        </div>
      </div>
    )
  },
)

export default TagsResultCard
