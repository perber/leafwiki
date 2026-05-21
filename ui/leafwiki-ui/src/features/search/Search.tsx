import { ListView, ListViewList, ListViewStatus } from '@/components/ListView'
import { Pagination } from '@/components/Pagination'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Input } from '@/components/ui/input'
import { searchPages, SearchResultItem, SearchTagFacet } from '@/lib/api/search'
import { deferStateUpdate } from '@/lib/deferState'
import { normalizeWikiRoutePath } from '@/lib/wikiPath'
import { fetchTags, TagCount } from '@/lib/api/tags'
import { useDebounce } from '@/lib/useDebounce'
import { X } from 'lucide-react'
import {
  startTransition,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import SearchResultCard from './SearchResultCard'

type SearchProps = {
  active?: boolean
}

export default function Search({ active = false }: SearchProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const urlQuery = searchParams.get('q') ?? ''
  const [inputQuery, setInputQuery] = useState(urlQuery)
  const activeTags = searchParams.getAll('tags')
  const activeTagsKey = activeTags.join('\n')
  const debouncedActiveTagsKey = useDebounce(activeTagsKey, 180)

  const [loading, setLoading] = useState(
    () => urlQuery.length >= 3 || activeTags.length > 0,
  )
  const [results, setResults] = useState<SearchResultItem[]>([])
  const [totalCount, setTotalCount] = useState(0)
  const [page, setPage] = useState(0)
  const [activeIndex, setActiveIndex] = useState(0)
  const [availableTags, setAvailableTags] = useState<TagCount[]>([])
  const [loadingAvailableTags, setLoadingAvailableTags] = useState(true)
  const [availableTagsError, setAvailableTagsError] = useState(false)
  const [facetTags, setFacetTags] = useState<SearchTagFacet[]>([])
  const searchInputRef = useRef<HTMLInputElement | null>(null)
  const resultRefs = useRef<(HTMLAnchorElement | null)[]>([])
  const latestRequestIdRef = useRef(0)

  const limit = 10
  const debouncedQuery = useDebounce(inputQuery, 300)
  const debouncedActiveTags = useMemo(
    () =>
      debouncedActiveTagsKey === '' ? [] : debouncedActiveTagsKey.split('\n'),
    [debouncedActiveTagsKey],
  )
  const hasSearchQuery = debouncedQuery.length >= 3
  const trimmedQuery = inputQuery.trim()
  const activeQueryLabel = hasSearchQuery ? debouncedQuery : trimmedQuery
  const isIdleMode = trimmedQuery === '' && activeTags.length === 0
  const hasImmediateFilters = !isIdleMode
  const hasDebouncedFilters = hasSearchQuery || debouncedActiveTags.length > 0
  const hasResults = results.length > 0
  const clampedActiveIndex =
    results.length === 0 ? 0 : Math.min(activeIndex, results.length - 1)
  const visibleTags = isIdleMode ? availableTags : facetTags
  const availableTagsLabel =
    activeTags.length === 0
      ? `${visibleTags.length} tag${visibleTags.length === 1 ? '' : 's'} available`
      : `${activeTags.length} selected tag${activeTags.length === 1 ? '' : 's'}`
  const activeFilterLabel = useMemo(() => {
    if (activeQueryLabel && activeTags.length > 0) {
      return `"${activeQueryLabel}" with ${activeTags.length} tag${activeTags.length === 1 ? '' : 's'}`
    }
    if (activeQueryLabel) {
      return `"${activeQueryLabel}"`
    }
    if (activeTags.length === 1) {
      return `tagged "${activeTags[0]}"`
    }
    return `tagged with ${activeTags.length} tags`
  }, [activeQueryLabel, activeTags])

  const invalidatePendingRequests = () => {
    latestRequestIdRef.current += 1
  }

  const toggleActiveTag = useCallback(
    (tag: string) => {
      invalidatePendingRequests()
      setLoading(true)
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          const current = next.getAll('tags')
          next.delete('tags')
          if (current.includes(tag)) {
            current
              .filter((existingTag) => existingTag !== tag)
              .forEach((existingTag) => next.append('tags', existingTag))
          } else {
            ;[...current, tag].forEach((existingTag) =>
              next.append('tags', existingTag),
            )
          }
          return next
        },
        { replace: true },
      )
      setPage(0)
      setActiveIndex(0)
    },
    [setSearchParams],
  )

  const clearActiveTags = useCallback(() => {
    invalidatePendingRequests()
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.delete('tags')
        return next
      },
      { replace: true },
    )
    setPage(0)
    setActiveIndex(0)
    if (urlQuery.trim().length < 3) {
      setResults([])
      setTotalCount(0)
      setFacetTags([])
      setLoading(false)
    }
  }, [setSearchParams, urlQuery])

  useEffect(() => {
    if (active) {
      searchInputRef.current?.focus()
    }
  }, [active])

  useEffect(() => {
    deferStateUpdate(() => {
      setInputQuery(urlQuery)
    })
  }, [urlQuery])

  useEffect(() => {
    if (!active) {
      return
    }

    let cancelled = false

    fetchTags('', 200)
      .then((tags) => {
        if (cancelled) return
        setAvailableTags(tags)
      })
      .catch(() => {
        if (cancelled) return
        setAvailableTags([])
        setAvailableTagsError(true)
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingAvailableTags(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [active])

  useEffect(() => {
    resultRefs.current = resultRefs.current.slice(0, results.length)
  }, [results.length])

  useEffect(() => {
    if (!hasResults) return

    resultRefs.current[clampedActiveIndex]?.scrollIntoView({
      block: 'nearest',
    })
  }, [clampedActiveIndex, hasResults])

  useEffect(() => {
    if (!hasDebouncedFilters) {
      return
    }

    const requestId = latestRequestIdRef.current + 1
    latestRequestIdRef.current = requestId

    searchPages(
      hasSearchQuery ? debouncedQuery : '',
      page * limit,
      limit,
      debouncedActiveTags,
    )
      .then((data) => {
        if (latestRequestIdRef.current !== requestId) return
        setResults(data.items || [])
        setTotalCount(data.count)
        setFacetTags(data.tag_facets || [])
      })
      .catch((err) => {
        if (latestRequestIdRef.current !== requestId) return
        console.error('Search failed', err)
        setResults([])
        setTotalCount(0)
        setFacetTags([])
      })
      .finally(() => {
        if (latestRequestIdRef.current !== requestId) return
        setLoading(false)
      })
  }, [
    debouncedActiveTags,
    debouncedQuery,
    hasDebouncedFilters,
    hasSearchQuery,
    page,
  ])

  const clearSearch = () => {
    invalidatePendingRequests()
    setInputQuery('')
    startTransition(() => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          next.delete('q')
          return next
        },
        { replace: true },
      )
    })
    setPage(0)
    setActiveIndex(0)
    if (activeTags.length === 0) {
      setResults([])
      setTotalCount(0)
      setFacetTags([])
      setLoading(false)
      return
    }

    setLoading(true)
  }

  const openActiveResult = () => {
    const activeResult = results[clampedActiveIndex]
    if (!activeResult) return

    navigate({
      pathname: normalizeWikiRoutePath(activeResult.path),
      search: location.search,
    })
  }

  return (
    <div className="search">
      <div className="search__input-wrapper">
        <Input
          ref={searchInputRef}
          autoFocus
          type="text"
          placeholder="Search..."
          value={inputQuery}
          data-testid="search-input"
          onChange={(e) => {
            const nextQuery = e.target.value
            invalidatePendingRequests()
            setInputQuery(nextQuery)
            startTransition(() => {
              setSearchParams(
                (prev) => {
                  const next = new URLSearchParams(prev)
                  if (nextQuery) {
                    next.set('q', nextQuery)
                  } else {
                    next.delete('q')
                  }
                  return next
                },
                { replace: true },
              )
            })
            setPage(0)
            setActiveIndex(0)

            if (nextQuery.length >= 3 || activeTags.length > 0) {
              setLoading(true)
            } else {
              setResults([])
              setTotalCount(0)
              setFacetTags([])
              setLoading(false)
            }
          }}
          onKeyDown={(e) => {
            if (e.key === 'ArrowDown') {
              if (!hasResults) return

              e.preventDefault()
              setActiveIndex((current) =>
                Math.min(current + 1, Math.max(results.length - 1, 0)),
              )
            }

            if (e.key === 'ArrowUp') {
              if (!hasResults) return

              e.preventDefault()
              setActiveIndex((current) => Math.max(current - 1, 0))
            }

            if (e.key === 'Enter') {
              if (!hasResults) return

              e.preventDefault()
              openActiveResult()
            }
          }}
          className="search__input"
        />
        {inputQuery && (
          <button
            onClick={clearSearch}
            className="search__clear-button"
            title="Clear"
            data-testid="search-clear-button"
          >
            <X size={16} />
          </button>
        )}
      </div>

      {(visibleTags.length > 0 ||
        loadingAvailableTags ||
        availableTagsError) && (
        <Accordion
          type="single"
          collapsible
          className="browse-tags__accordion"
          data-testid="search-tags-accordion"
        >
          <AccordionItem
            value="all-tags"
            className="browse-tags__accordion-item"
          >
            <AccordionTrigger
              className="browse-tags__accordion-trigger"
              data-testid="search-tags-accordion-trigger"
            >
              <span className="browse-tags__accordion-title">All Tags</span>
              <span className="browse-tags__accordion-summary">
                {availableTagsLabel}
              </span>
            </AccordionTrigger>
            <AccordionContent className="browse-tags__accordion-content">
              {loadingAvailableTags ? (
                <p className="browse-tags__accordion-empty">Loading tags…</p>
              ) : availableTagsError ? (
                <p
                  className="browse-tags__accordion-empty"
                  data-testid="tags-available-error"
                >
                  Failed to load tags.
                </p>
              ) : (
                <div
                  className="browse-tags__tag-list"
                  data-testid="search-tags-list"
                >
                  {visibleTags.map(({ tag, count }) => {
                    const isActive = activeTags.includes(tag)
                    return (
                      <button
                        key={tag}
                        type="button"
                        className={`browse-tags__tag-filter ${isActive ? 'browse-tags__tag-filter--active' : ''}`.trim()}
                        onClick={() => toggleActiveTag(tag)}
                        aria-pressed={isActive}
                        data-testid={`tags-filter-${tag}`}
                      >
                        <span className="browse-tags__tag-filter-label">
                          {tag}
                        </span>
                        <span className="browse-tags__tag-filter-count">
                          {count}
                        </span>
                      </button>
                    )
                  })}
                </div>
              )}
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      )}

      <div className="search__body">
        {hasImmediateFilters && (
          <div className="browse-results__toolbar">
            <ListViewStatus className="search__result-summary">
              <span className="browse-results__summary">
                <span className="browse-results__summary-count">
                  Found <strong>{totalCount}</strong>
                </span>
                <span className="browse-results__summary-text">
                  {` result${totalCount !== 1 ? 's' : ''} for `}
                  <strong>{activeFilterLabel}</strong>
                </span>
              </span>
            </ListViewStatus>
            {activeTags.length > 0 && (
              <button
                type="button"
                className="browse-results__clear"
                onClick={clearActiveTags}
                title="Clear tag filter"
                data-testid="search-tags-clear-button"
              >
                <X size={12} />
              </button>
            )}
          </div>
        )}

        {loading && hasImmediateFilters && (
          <ListView
            as="div"
            className="search__results-view"
            contentClassName="search__content"
          >
            <ListViewStatus className="search__result-summary">
              Loading results...
            </ListViewStatus>
          </ListView>
        )}

        {!loading && hasImmediateFilters && results.length === 0 && (
          <ListView
            as="div"
            className="search__results-view"
            contentClassName="search__content"
          >
            <ListViewStatus className="search__result-summary">
              No results found.
            </ListViewStatus>
          </ListView>
        )}

        {!loading && results.length > 0 && (
          <ListView
            as="div"
            className="search__results-view"
            contentClassName="search__content"
            testId="search-results-list"
            footer={
              <div className="search__pagination">
                <Pagination
                  total={totalCount}
                  page={page}
                  limit={limit}
                  onPageChange={(newPage) => {
                    invalidatePendingRequests()
                    setLoading(true)
                    setPage(newPage)
                    setActiveIndex(0)
                  }}
                />
              </div>
            }
          >
            <ListViewList>
              {results.map((item, index) => (
                <SearchResultCard
                  key={`${item.page_id}-${item.kind}-${index}`}
                  ref={(element) => {
                    resultRefs.current[index] = element
                  }}
                  item={item}
                  isSelected={index === clampedActiveIndex}
                  onMouseEnter={() => setActiveIndex(index)}
                  onFocus={() => setActiveIndex(index)}
                />
              ))}
            </ListViewList>
          </ListView>
        )}
      </div>
    </div>
  )
}
