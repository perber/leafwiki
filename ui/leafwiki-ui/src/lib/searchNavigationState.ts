export const SEARCH_QUERY_STATE_KEY = 'leafwikiSearchQuery'

export function getNavigationSearchQuery(state: unknown): string | undefined {
  if (typeof state === 'object' && state !== null) {
    const s = state as Record<string, unknown>
    const q = s[SEARCH_QUERY_STATE_KEY]
    if (typeof q === 'string' && q.length > 0) return q
  }
  return undefined
}
