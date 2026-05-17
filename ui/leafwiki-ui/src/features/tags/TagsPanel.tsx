import { ListView, ListViewList, ListViewStatus } from '@/components/ListView'
import { Pagination } from '@/components/Pagination'
import TagInputWithSuggestions from '@/components/TagInputWithSuggestions'
import {
  fetchPagesByTags,
  fetchTags,
  TagCount,
  TaggedPage,
} from '@/lib/api/tags'
import { useTagsStore } from '@/stores/tags'
import { X } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import TagsResultCard from './TagsResultCard'

type TagsPanelProps = {
  active?: boolean
}

export default function TagsPanel({ active = false }: TagsPanelProps) {
  const navigate = useNavigate()
  const activeTags = useTagsStore((s) => s.activeTags)
  const setActiveTags = useTagsStore((s) => s.setActiveTags)
  const clearActiveTags = useTagsStore((s) => s.clearActiveTags)
  const toggleActiveTag = useTagsStore((s) => s.toggleActiveTag)

  const [results, setResults] = useState<TaggedPage[]>([])
  const [availableTags, setAvailableTags] = useState<TagCount[]>([])
  const [contextualTags, setContextualTags] = useState<TagCount[]>([])
  const [loadingAvailableTags, setLoadingAvailableTags] = useState(false)
  const [availableTagsError, setAvailableTagsError] = useState(false)
  const [loadingResults, setLoadingResults] = useState(false)
  const [fetchError, setFetchError] = useState(false)
  const [page, setPage] = useState(0)
  const [activeIndex, setActiveIndex] = useState(0)
  const resultRefs = useRef<(HTMLDivElement | null)[]>([])
  const resultsPerPage = 10

  useEffect(() => {
    if (!active) {
      return
    }

    let cancelled = false

    const loadAvailableTags = async () => {
      setLoadingAvailableTags(true)
      setAvailableTagsError(false)

      try {
        const tags = await fetchTags('', 200)
        if (cancelled) return
        setAvailableTags(tags)
      } catch {
        if (cancelled) return
        setAvailableTags([])
        setAvailableTagsError(true)
      } finally {
        if (!cancelled) {
          setLoadingAvailableTags(false)
        }
      }
    }

    void loadAvailableTags()

    return () => {
      cancelled = true
    }
  }, [active])

  useEffect(() => {
    if (!active || activeTags.length === 0) {
      setContextualTags([])
      return
    }

    let cancelled = false

    const loadContextualTags = async () => {
      try {
        const tags = await fetchTags('', 200, activeTags)
        if (cancelled) return
        setContextualTags(tags)
      } catch {
        if (cancelled) return
        setContextualTags([])
      }
    }

    void loadContextualTags()

    return () => {
      cancelled = true
    }
  }, [active, activeTags])

  useEffect(() => {
    if (activeTags.length === 0) {
      setResults([])
      setLoadingResults(false)
      setFetchError(false)
      setPage(0)
      setActiveIndex(0)
      return
    }

    const controller = new AbortController()

    const loadResults = async () => {
      setLoadingResults(true)
      setFetchError(false)
      try {
        const pages = await fetchPagesByTags(activeTags, controller.signal)
        setResults(pages)
        setPage(0)
        setActiveIndex(0)
      } catch (e) {
        if ((e as Error).name !== 'AbortError') {
          setFetchError(true)
        }
      } finally {
        setLoadingResults(false)
      }
    }

    void loadResults()
    return () => controller.abort()
  }, [activeTags])

  const activeTagsLabel =
    activeTags.length === 1
      ? `Tagged "${activeTags[0]}"`
      : `Tagged with ${activeTags.length} tags`

  const paginatedResults = useMemo(
    () => results.slice(page * resultsPerPage, (page + 1) * resultsPerPage),
    [page, results],
  )
  const hasResults = paginatedResults.length > 0
  const clampedActiveIndex =
    paginatedResults.length === 0
      ? 0
      : Math.min(activeIndex, paginatedResults.length - 1)
  const showInitialResultsLoading = loadingResults && results.length === 0
  const showResultsRefreshing = loadingResults && results.length > 0
  const showError = fetchError && !loadingResults
  const contextualTagCounts = useMemo(
    () => new Map(contextualTags.map(({ tag, count }) => [tag, count])),
    [contextualTags],
  )
  const visibleTags = useMemo(
    () =>
      availableTags.map(({ tag, count }) => {
        if (activeTags.length === 0) {
          return { tag, count, isRelated: true }
        }

        if (activeTags.includes(tag)) {
          return { tag, count: results.length, isRelated: true }
        }

        const contextualCount = contextualTagCounts.get(tag) ?? 0
        return { tag, count: contextualCount, isRelated: contextualCount > 0 }
      }),
    [activeTags, availableTags, contextualTagCounts, results.length],
  )
  const relatedTagCount =
    activeTags.length === 0
      ? availableTags.length
      : visibleTags.filter((tag) => tag.isRelated).length
  const availableTagsLabel =
    activeTags.length === 0
      ? `${availableTags.length} tag${availableTags.length === 1 ? '' : 's'} available`
      : `${relatedTagCount} related tag${relatedTagCount === 1 ? '' : 's'}`

  useEffect(() => {
    resultRefs.current = resultRefs.current.slice(0, paginatedResults.length)
  }, [paginatedResults.length])

  useEffect(() => {
    if (!hasResults) {
      return
    }

    resultRefs.current[clampedActiveIndex]?.scrollIntoView({
      block: 'nearest',
    })
  }, [clampedActiveIndex, hasResults])

  const openActiveResult = () => {
    const activeResult = paginatedResults[clampedActiveIndex]
    if (!activeResult) {
      return
    }

    navigate(`/${activeResult.path}`)
  }

  return (
    <div className="tags-panel">
      <div className="browse-tags__search search__input-wrapper">
        <TagInputWithSuggestions
          tags={activeTags}
          onTagsChange={(tags) => {
            setActiveTags(tags)
            setPage(0)
            setActiveIndex(0)
          }}
          placeholder={
            activeTags.length === 0 ? 'Add tags to filter…' : 'Add another tag…'
          }
          variant="browse"
          inputTestId="tags-search-input"
          active={active}
          onArrowDown={() => {
            if (!hasResults) return false
            setActiveIndex((current) =>
              Math.min(current + 1, Math.max(paginatedResults.length - 1, 0)),
            )
            return true
          }}
          onArrowUp={() => {
            if (!hasResults) return false
            setActiveIndex((current) => Math.max(current - 1, 0))
            return true
          }}
          onSubmitWithoutSuggestion={() => {
            if (hasResults) {
              openActiveResult()
            }
          }}
        />
      </div>

      <Accordion type="single" collapsible className="browse-tags__accordion">
        <AccordionItem value="all-tags" className="browse-tags__accordion-item">
          <AccordionTrigger className="browse-tags__accordion-trigger">
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
            ) : availableTags.length === 0 ? (
              <p className="browse-tags__accordion-empty">No tags found.</p>
            ) : (
              <div
                className="browse-tags__tag-list"
                data-testid="tags-all-list"
              >
                {visibleTags.map(({ tag, count, isRelated }) => {
                  const isActive = activeTags.includes(tag)
                  return (
                    <button
                      key={tag}
                      type="button"
                      className={`browse-tags__tag-filter ${isActive ? 'browse-tags__tag-filter--active' : ''} ${!isActive && !isRelated ? 'browse-tags__tag-filter--muted' : ''}`.trim()}
                      onClick={() => {
                        toggleActiveTag(tag)
                        setPage(0)
                        setActiveIndex(0)
                      }}
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

      <div className="browse-results search__body">
        {activeTags.length > 0 && (
          <>
            <div className="browse-results__toolbar">
              <ListViewStatus className="search__result-summary">
                <span className="browse-results__summary">
                  <span className="browse-results__summary-count">
                    Found <strong>{results.length}</strong>
                  </span>
                  <span className="browse-results__summary-text">
                    {`result${results.length !== 1 ? 's' : ''} for `}
                    <strong>{activeTagsLabel}</strong>
                  </span>
                </span>
                {showResultsRefreshing && (
                  <span className="browse-results__loading-indicator">
                    Updating…
                  </span>
                )}
              </ListViewStatus>
              <button
                type="button"
                className="browse-results__clear"
                onClick={() => {
                  clearActiveTags()
                }}
                title="Clear filter"
              >
                <X size={12} />
              </button>
            </div>
            {showInitialResultsLoading ? (
              <ListView
                as="div"
                className="browse-results__view search__results-view"
                contentClassName="search__content"
              >
                <ListViewStatus className="browse-results__empty">
                  Loading…
                </ListViewStatus>
              </ListView>
            ) : showError ? (
              <ListView
                as="div"
                className="browse-results__view search__results-view"
                contentClassName="search__content"
              >
                <ListViewStatus
                  className="browse-results__empty"
                  data-testid="tags-fetch-error"
                >
                  Failed to load results. Please try again.
                </ListViewStatus>
              </ListView>
            ) : results.length === 0 ? (
              <ListView
                as="div"
                className="browse-results__view search__results-view"
                contentClassName="search__content"
              >
                <ListViewStatus className="browse-results__empty">
                  No pages found.
                </ListViewStatus>
              </ListView>
            ) : (
              <ListView
                as="div"
                className={`browse-results__view search__results-view ${
                  showResultsRefreshing ? 'browse-results__view--loading' : ''
                }`.trim()}
                contentClassName="search__content"
                testId="tags-results-list"
                footer={
                  <div className="browse-results__pagination search__pagination">
                    <Pagination
                      total={results.length}
                      page={page}
                      limit={resultsPerPage}
                      onPageChange={(newPage) => {
                        setPage(newPage)
                        setActiveIndex(0)
                      }}
                    />
                  </div>
                }
              >
                <ListViewList>
                  {paginatedResults.map((resultPage, index) => (
                    <TagsResultCard
                      key={resultPage.id}
                      ref={(element) => {
                        resultRefs.current[index] = element
                      }}
                      item={resultPage}
                      activeTags={activeTags}
                      isSelected={index === clampedActiveIndex}
                      onMouseEnter={() => setActiveIndex(index)}
                      onFocus={() => setActiveIndex(index)}
                      onTagClick={toggleActiveTag}
                    />
                  ))}
                </ListViewList>
              </ListView>
            )}
          </>
        )}
      </div>
    </div>
  )
}
