/* eslint-disable react-hooks/set-state-in-effect */
import { Pagination } from '@/components/Pagination'
import { searchPages, SearchResultItem } from '@/lib/api/search'
import { useDebounce } from '@/lib/useDebounce'
import { X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import SearchResultCard from './SearchResultCard'

type SearchProps = {
  active?: boolean
}

export default function Search({ active = false }: SearchProps) {
  const [query, setQuery] = useState('')
  const [limit, setLimit] = useState<number>(10)
  const [loading, setLoading] = useState<boolean>(false)
  const [totalCount, setTotalCount] = useState<number>(0)
  const [results, setResults] = useState<SearchResultItem[]>([])
  const [page, setPage] = useState(0) // 0-based
  const searchInputRef = useRef<HTMLInputElement | null>(null)

  const debouncedQuery = useDebounce(query, 300)

  useEffect(() => {
    if (active) {
      searchInputRef.current?.focus()
    }
  }, [active])

  useEffect(() => {
    if (debouncedQuery) {
      if (debouncedQuery.length < 3) {
        setResults([])
        setTotalCount(0)
        return
      }
      setLoading(true)
      searchPages(debouncedQuery, page * limit, limit)
        .then((data) => {
          setResults(data.items)
          setTotalCount(data.count)
        })
        .catch((err) => console.error('Search failed', err))
        .finally(() => setLoading(false))
    }

    if (!debouncedQuery) {
      setResults([])
      setLimit(10)
      setPage(0)
      setTotalCount(0)
    }
  }, [debouncedQuery, limit, page])

  const clearSearch = () => {
    setQuery('')
    setPage(0)
  }

return (
    <div className="search">
      <div className="search__input-wrapper">
        <input
          ref={searchInputRef}
          autoFocus
          type="text"
          placeholder="Search..."
          value={query}
          data-testid="search-input"
          onChange={(e) => setQuery(e.target.value)}
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

        {!loading && query && results.length === 0 && (
          <div className="search__status search__status--empty">
            No results found for "<strong>{query}</strong>"
          </div>
        )}

        {!loading && results.length > 0 && (
          <div className="search__result-summary">
            Found <strong>{totalCount}</strong> result
            {totalCount !== 1 ? 's' : ''} for "<strong>{query}</strong>"
          </div>
        )}

        {!loading && results.length > 0 && (
          <>
            <div className="search__results">
              {results.map((item) => {
                if (item.page_id && item.path && item.title) {
                  return <SearchResultCard key={item.page_id} item={item} />
                }
                return <></>
              })}
            </div>
            <Pagination
              total={totalCount}
              page={page}
              limit={limit}
              onPageChange={(newPage) => setPage(newPage)}
            />
          </>
        )}
      </div>
    </div>
  )
}