import { Pagination } from '@/components/Pagination'
import { searchPages, SearchResultItem } from '@/lib/api'
import { useDebounce } from '@/lib/useDebounce'
import { X } from 'lucide-react'
import { useEffect, useState } from 'react'
import SearchResultCard from './SearchResultCard'

export default function Search() {
  const [query, setQuery] = useState('')
  const [limit, setLimit] = useState<number>(10)
  const [loading, setLoading] = useState<boolean>(false)
  const [totalCount, setTotalCount] = useState<number>(0)
  const [results, setResults] = useState<SearchResultItem[]>([])
  const [page, setPage] = useState(0) // 0-based

  const debouncedQuery = useDebounce(query, 300)

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
    <div className="mt-2">
      <div className="relative mb-4">
        <input
          autoFocus
          type="text"
          placeholder="Search..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="w-full rounded border px-2 py-1 pr-8"
        />
        {query && (
          <button
            onClick={clearSearch}
            className="absolute top-1/2 right-1 -translate-y-1/2 text-sm text-gray-500 hover:text-black"
            title="Clear"
          >
            <X size={16} />
          </button>
        )}
      </div>
      <div className="mb-4">
        {loading && (
          <div className="text-sm text-gray-500">Loading results...</div>
        )}

        {!loading && query && results.length === 0 && (
          <div className="text-sm text-gray-500">
            No results found for "<strong>{query}</strong>"
          </div>
        )}

        {!loading && results.length > 0 && (
          <div className="mb-2 text-sm">
            Found <strong>{totalCount}</strong> result
            {totalCount !== 1 ? 's' : ''} for "<strong>{query}</strong>"
          </div>
        )}

        {!loading && results.length > 0 && (
          <>
            <div className="space-y-4">
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
