import i18next from '@/lib/i18n'
import { Trans } from 'react-i18next'
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
import { getShortcutDefinition } from '@/lib/shortcuts/shortcutCatalog'
import { useSidebarStore } from '@/stores/sidebar'
import { X } from 'lucide-react'
import {
  startTransition,
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useLocation, useNavigate, useSearchParams } from 'react-router'
import SearchResultCard from './SearchResultCard'
import {
  SearchSyntaxHelpButton,
  SearchSyntaxHelpPanel,
} from './SearchSyntaxHelp'
import { useCloseSyntaxHelpOnEscape } from './useCloseSyntaxHelpOnEscape'

type SearchProps = {
  active?: boolean
}

const t = (key: string, opts?: Record<string, unknown>) =>
  i18next.t(key, { ...opts, ns: 'search' })

// Sidebar panel switching must keep working while the search field is
// focused, otherwise the shortcuts are only reachable from elsewhere on
// the page (see HotKeyHandler's input/button focus guard).
const SEARCH_INPUT_ALLOWED_HOTKEYS = [
  getShortcutDefinition('sidebar.explorer.open').keyCombo,
  getShortcutDefinition('sidebar.search.open').keyCombo,
].join(' ')

// Shorter queries are not sent: a single character would prefix-match
// almost every page, while two already cover common wiki words (Pi, HR,
// IP, Go, C#).
const MIN_QUERY_LENGTH = 2

const isSearchableQuery = (query: string) =>
  query.trim().length >= MIN_QUERY_LENGTH

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
    () => isSearchableQuery(urlQuery) || activeTags.length > 0,
  )
  const [results, setResults] = useState<SearchResultItem[]>([])
  const [totalCount, setTotalCount] = useState(0)
  const [page, setPage] = useState(0)
  const [activeIndex, setActiveIndex] = useState(0)
  const [availableTags, setAvailableTags] = useState<TagCount[]>([])
  const [availableTagsError, setAvailableTagsError] = useState(false)
  const [facetTags, setFacetTags] = useState<SearchTagFacet[]>([])
  const searchInputRef = useRef<HTMLInputElement | null>(null)
  // Like the query and the tag accordion, the help panel keeps its state
  // while the sidebar shows another panel (Search stays mounted, hidden).
  const [syntaxHelpOpen, setSyntaxHelpOpen] = useState(false)
  const syntaxHelpPanelId = useId()
  const syntaxHelpButtonRef = useRef<HTMLButtonElement | null>(null)
  useCloseSyntaxHelpOnEscape({
    enabled: syntaxHelpOpen && active,
    setOpen: setSyntaxHelpOpen,
    buttonRef: syntaxHelpButtonRef,
  })
  const searchFocusRequestId = useSidebarStore(
    (state) => state.searchFocusRequestId,
  )
  const resultRefs = useRef<(HTMLAnchorElement | null)[]>([])
  const latestRequestIdRef = useRef(0)
  // Bumped when a keystroke invalidates the pending request but leaves the
  // debounced query unchanged ("Pi" -> "P" -> "Pi" within the debounce), so
  // the request effect still runs and settles the loading state.
  const [retypedQueryCount, setRetypedQueryCount] = useState(0)

  const limit = 10
  const debouncedQuery = useDebounce(inputQuery, 300)
  const debouncedActiveTags = useMemo(
    () =>
      debouncedActiveTagsKey === '' ? [] : debouncedActiveTagsKey.split('\n'),
    [debouncedActiveTagsKey],
  )
  const hasSearchQuery = isSearchableQuery(debouncedQuery)
  const trimmedQuery = inputQuery.trim()
  const isQueryTooShort = trimmedQuery !== '' && !isSearchableQuery(inputQuery)
  const activeQueryLabel = isQueryTooShort
    ? ''
    : hasSearchQuery
      ? debouncedQuery
      : trimmedQuery
  const isIdleMode = trimmedQuery === '' && activeTags.length === 0
  // A too-short query on its own filters nothing; the hint replaces the
  // result list instead of an empty or stale one.
  const hasImmediateFilters =
    !isIdleMode && (!isQueryTooShort || activeTags.length > 0)
  const hasDebouncedFilters = hasSearchQuery || debouncedActiveTags.length > 0
  const hasResults = results.length > 0
  const clampedActiveIndex =
    results.length === 0 ? 0 : Math.min(activeIndex, results.length - 1)
  const visibleTags = isIdleMode ? availableTags : facetTags
  const availableTagsLabel =
    activeTags.length === 0
      ? t('tags.available', { count: visibleTags.length })
      : t('tags.selected', { count: activeTags.length })
  const activeFilterLabel = useMemo(() => {
    if (activeQueryLabel && activeTags.length > 0) {
      return t('results.filterQueryWithTags', {
        query: activeQueryLabel,
        count: activeTags.length,
      })
    }
    if (activeQueryLabel) {
      return t('results.filterQuery', { query: activeQueryLabel })
    }
    if (activeTags.length === 1) {
      return t('results.filterTagged', { tag: activeTags[0] })
    }
    return t('results.filterTaggedWith', { count: activeTags.length })
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
    if (!isSearchableQuery(urlQuery)) {
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
    // searchFocusRequestId is bumped when the search shortcut is pressed
    // again while already active, to refocus the input on demand.
  }, [active, searchFocusRequestId])

  useEffect(() => {
    if (!active) {
      return
    }

    deferStateUpdate(() => {
      setInputQuery((currentQuery) =>
        currentQuery === urlQuery ? currentQuery : urlQuery,
      )
    })
  }, [active, urlQuery])

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
    retypedQueryCount,
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
      <div className="search__input-row">
        <div className="search__input-wrapper">
          <Input
            ref={searchInputRef}
            autoFocus
            type="text"
            placeholder={t('input.placeholder')}
            value={inputQuery}
            data-testid="search-input"
            data-allow-hotkeys={SEARCH_INPUT_ALLOWED_HOTKEYS}
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

              if (isSearchableQuery(nextQuery) || activeTags.length > 0) {
                setLoading(true)
                if (nextQuery === debouncedQuery) {
                  setRetypedQueryCount((count) => count + 1)
                }
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
            // pr-8 keeps long queries clear of the clear button. It is a
            // utility here because the Input's own px-3 is one and would beat
            // padding set in .search__input (components layer).
            className="search__input pr-8"
          />
          {inputQuery && (
            <button
              onClick={clearSearch}
              className="search__clear-button"
              title={t('input.clearButton')}
              data-testid="search-clear-button"
            >
              <X size={16} />
            </button>
          )}
        </div>
        <SearchSyntaxHelpButton
          ref={syntaxHelpButtonRef}
          open={syntaxHelpOpen}
          panelId={syntaxHelpPanelId}
          onToggle={() => setSyntaxHelpOpen((open) => !open)}
        />
      </div>
      <SearchSyntaxHelpPanel id={syntaxHelpPanelId} open={syntaxHelpOpen} />

      {(visibleTags.length > 0 || availableTagsError) && (
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
              <span className="browse-tags__accordion-title">
                {t('tags.accordionTitle')}
              </span>
              <span className="browse-tags__accordion-summary">
                {availableTagsLabel}
              </span>
            </AccordionTrigger>
            <AccordionContent className="browse-tags__accordion-content">
              {availableTagsError ? (
                <p
                  className="browse-tags__accordion-empty"
                  data-testid="tags-available-error"
                >
                  {t('tags.error')}
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
        {/* Always rendered so screen readers announce the hint when its
            text appears; a live region inserted with its text is often
            missed. */}
        <p role="status" className="search__min-length-hint">
          {isQueryTooShort && (
            <span data-testid="search-min-length-hint">
              {t('input.minLengthHint', { count: MIN_QUERY_LENGTH })}
            </span>
          )}
        </p>

        {hasImmediateFilters && (
          <div className="browse-results__toolbar">
            <ListViewStatus className="search__result-summary">
              <span className="browse-results__summary">
                <span className="browse-results__summary-count">
                  <Trans
                    i18nKey="results.resultSummary"
                    count={totalCount}
                    ns="search"
                    components={{ bold: <strong /> }}
                  />
                </span>
                <span className="browse-results__summary-text">
                  {' '}
                  <strong>{activeFilterLabel}</strong>
                </span>
              </span>
            </ListViewStatus>
            {activeTags.length > 0 && (
              <button
                type="button"
                className="browse-results__clear"
                onClick={clearActiveTags}
                title={t('tags.clearFilter')}
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
              {t('results.loading')}
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
              {t('results.noResults')}
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
