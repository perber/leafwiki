import { fetchWithAuth } from './auth'

export type IndexingStatus = {
  active: boolean
  indexed: number
  failed: number
  finished_at: string
}

export type SearchResultItem = {
  page_id: string
  path: string
  title: string
  excerpt: string
  rank: number
}

export type SearchResult = {
  count: number
  items: SearchResultItem[]
  limit: number
  offset: number
}

export async function searchPages(
  query: string,
  offset: number,
  limit: number,
): Promise<SearchResult> {
  if (offset < 0) offset = 0
  if (limit < 1 || limit > 100) limit = 10

  if (!query) return { count: 0, items: [], limit: 10, offset: 0 }

  const data = await fetchWithAuth(
    `/api/search?q=${encodeURIComponent(query)}&offset=${offset}&limit=${limit}`,
  )

  return data as SearchResult
}

export async function getSearchStatus(): Promise<IndexingStatus> {
  const res = await fetchWithAuth('/api/search/status')
  return res as IndexingStatus
}
