import { Pagination } from '@/components/Pagination'
import { Input } from '@/components/ui/input'
import { searchPages, SearchResultItem } from '@/lib/api/search'
import { useDebounce } from '@/lib/useDebounce'
import { X } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import SearchResultCard from './SearchResultCard'

type SearchProps = {
  active?: boolean
}

export default function Search({ active = false }: SearchProps) {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState<boolean>(false)
  const [totalCount, setTotalCount] = useState<number>(0)
  const [results, setResults] = useState<SearchResultItem[]>([])
  const [page, setPage] = useState(0) // 0-based
  const [activeIndex, setActiveIndex] = useState(0)
  const searchInputRef = useRef<HTMLInputElement | null>(null)
  const resultRefs = useRef<(HTMLAnchorElement | null)[]>([])
  const latestRequestIdRef = useRef(0)

  const limit = 10
  const debouncedQuery = useDebounce(query, 300)
  const hasSearchQuery = debouncedQuery.length >= 3
  const visibleResults = useMemo(
    () => (hasSearchQuery ? results : []),
    [hasSearchQuery, results],
  )
  const visibleTotalCount = hasSearchQuery ? totalCount : 0
  const hasResults = visibleResults.length > 0
  const clampedActiveIndex =
    visibleResults.length === 0
      ? 0
      : Math.min(activeIndex, visibleResults.length - 1)

  useEffect(() => {
    if (active) {
      searchInputRef.current?.focus()
    }
  }, [active])

  useEffect(() => {
    resultRefs.current = resultRefs.current.slice(0, visibleResults.length)
  }, [visibleResults])

  useEffect(() => {
    if (!hasResults) return

    resultRefs.current[clampedActiveIndex]?.scrollIntoView({
      block: 'nearest',
    })
  }, [clampedActiveIndex, hasResults])

  useEffect(() => {
    if (!hasSearchQuery) {
      return
    }

    const requestId = latestRequestIdRef.current + 1
    latestRequestIdRef.current = requestId

    searchPages(debouncedQuery, page * limit, limit)
      .then((data) => {
        if (latestRequestIdRef.current !== requestId) return

        setResults(data.items || [])
        setTotalCount(data.count)
      })
      .catch((err) => {
        if (latestRequestIdRef.current !== requestId) return

        console.error('Search failed', err)
        setResults([])
        setTotalCount(0)
      })
      .finally(() => {
        if (latestRequestIdRef.current !== requestId) return

        setLoading(false)
      })
  }, [debouncedQuery, hasSearchQuery, limit, page])

  const invalidatePendingRequests = () => {
    latestRequestIdRef.current += 1
  }

  const clearSearch = () => {
    invalidatePendingRequests()
    setQuery('')
    setPage(0)
    setActiveIndex(0)
    setResults([])
    setTotalCount(0)
    setLoading(false)
  }

  const openActiveResult = () => {
    const activeResult = results[clampedActiveIndex]
    if (!activeResult) return

    navigate(`${activeResult.path}`)
  }

  return (
    <div className="search">
      <div className="search__input-wrapper">
        <Input
          ref={searchInputRef}
          autoFocus
          type="text"
          placeholder="Search..."
          value={query}
          data-testid="search-input"
          onChange={(e) => {
            const nextQuery = e.target.value
            invalidatePendingRequests()
            setQuery(nextQuery)
            setPage(0)
            setActiveIndex(0)

            if (nextQuery.length >= 3) {
              setLoading(true)
            } else {
              setResults([])
              setTotalCount(0)
              setLoading(false)
            }
          }}
          onKeyDown={(e) => {
            if (e.key === 'ArrowDown') {
              if (!hasResults) return

              e.preventDefault()
              setActiveIndex((current) =>
                Math.min(current + 1, Math.max(visibleResults.length - 1, 0)),
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
        {query && (
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
      <div className="search__body">
        {loading && (
          <div className="search__status search__status--loading">
            Loading results...
          </div>
        )}

        {!loading && query && visibleResults.length === 0 && (
          <div className="search__status search__status--empty">
            No results found for "<strong>{query}</strong>"
          </div>
        )}

        {!loading && visibleResults.length > 0 && (
          <div className="search__result-summary">
            Found <strong>{visibleTotalCount}</strong> result
            {visibleTotalCount !== 1 ? 's' : ''} for "<strong>{query}</strong>"
          </div>
        )}

        {!loading && visibleResults.length > 0 && (
          <>
            <div className="search__results">
              {visibleResults.map((item, index) => {
                if (item.page_id && item.path && item.title) {
                  return (
                    <SearchResultCard
                      key={item.page_id}
                      ref={(element) => {
                        resultRefs.current[index] = element
                      }}
                      item={item}
                      isSelected={index === clampedActiveIndex}
                    />
                  )
                }
                return null
              })}
            </div>
            <Pagination
              total={visibleTotalCount}
              page={page}
              limit={limit}
              onPageChange={(newPage) => {
                invalidatePendingRequests()
                setLoading(true)
                setPage(newPage)
                setActiveIndex(0)
              }}
            />
          </>
        )}
      </div>
    </div>
  )
}
