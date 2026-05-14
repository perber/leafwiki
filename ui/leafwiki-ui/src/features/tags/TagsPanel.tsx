import { ListView, ListViewList, ListViewStatus } from '@/components/ListView'
import { Pagination } from '@/components/Pagination'
import TagInputWithSuggestions from '@/components/TagInputWithSuggestions'
import { fetchPagesByTags, TaggedPage } from '@/lib/api/tags'
import { useTagsStore } from '@/stores/tags'
import { X } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
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
  const [loadingResults, setLoadingResults] = useState(false)
  const [page, setPage] = useState(0)
  const [activeIndex, setActiveIndex] = useState(0)
  const resultRefs = useRef<(HTMLDivElement | null)[]>([])
  const resultsPerPage = 10

  useEffect(() => {
    if (activeTags.length === 0) {
      return
    }

    const loadResults = async () => {
      setLoadingResults(true)
      try {
        const pages = await fetchPagesByTags(activeTags)
        setResults(pages)
        setPage(0)
        setActiveIndex(0)
      } finally {
        setLoadingResults(false)
      }
    }

    void loadResults()
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
            if (tags.length === 0) {
              setResults([])
            }
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
                  setResults([])
                  setPage(0)
                  setActiveIndex(0)
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
