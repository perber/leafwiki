import { fetchWithAuth } from './auth'
import type { Page } from './pages'

export async function getFavorites(): Promise<Page[]> {
  const res = (await fetchWithAuth('/api/favorites')) as { pages: Page[] }
  return res.pages
}

export async function addFavorite(pageId: string): Promise<void> {
  await fetchWithAuth(`/api/pages/${pageId}/favorite`, { method: 'PUT' })
}

export async function removeFavorite(pageId: string): Promise<void> {
  await fetchWithAuth(`/api/pages/${pageId}/favorite`, { method: 'DELETE' })
}
