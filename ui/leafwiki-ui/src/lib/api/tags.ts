import { fetchWithAuth } from './auth'

export type TagCount = {
  tag: string
  count: number
}

export type TaggedPage = {
  id: string
  title: string
  path: string
  excerpt?: string
  tags: string[]
  updatedAt?: string
  lastAuthor?: { id: string; username: string }
}

export async function fetchTags(
  filter = '',
  limit = 50,
  selected: string[] = [],
): Promise<TagCount[]> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (filter) params.set('q', filter)
  if (selected.length > 0) params.set('selected', selected.join(','))
  return (await fetchWithAuth(`/api/tags?${params}`)) as TagCount[]
}

export async function fetchPagesByTags(tags: string[]): Promise<TaggedPage[]> {
  const params = new URLSearchParams({ tags: tags.join(',') })
  return (await fetchWithAuth(`/api/tags/pages?${params}`)) as TaggedPage[]
}
