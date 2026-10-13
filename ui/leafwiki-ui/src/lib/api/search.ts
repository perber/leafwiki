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
  kind: 'page' | 'section'
  excerpt: string
  rank: number
  tags: string[]
}

export type SearchTagFacet = {
  tag: string
  count: number
}

export type SearchResult = {
  count: number
  items: SearchResultItem[] | null
  limit: number
  offset: number
  tag_facets: SearchTagFacet[]
}

export async function searchPages(
  query: string,
  offset: number,
  limit: number,
  tags: string[] = [],
): Promise<SearchResult> {
  if (offset < 0) offset = 0
  if (limit < 1 || limit > 100) limit = 10

  if (!query && tags.length === 0) {
    return { count: 0, items: [], limit: 10, offset: 0, tag_facets: [] }
  }

  const params = new URLSearchParams({
    offset: String(offset),
    limit: String(limit),
  })
  if (query) params.set('q', query)
  for (const tag of tags) {
    params.append('tags', tag)
  }

  return await fetchWithAuth<SearchResult>(`/api/search?${params}`)
}

export async function getSearchStatus(): Promise<IndexingStatus> {
  return await fetchWithAuth<IndexingStatus>('/api/search/status')
}
