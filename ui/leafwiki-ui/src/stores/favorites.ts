// stores/favorites.ts
// A logged-in user's private set of favorited pages. Unlike Pinned Pages
// (global, part of the tree), this is per-user server truth fetched once via
// GET /api/favorites and never persisted to localStorage — it must be
// re-fetched per session and cleared on logout so a second user on the same
// browser never sees the first user's favorites. App.tsx owns calling
// loadFavorites()/clearFavorites() as the session's logged-in state changes.

import {
  addFavorite as addFavoriteAPI,
  getFavorites,
  removeFavorite as removeFavoriteAPI,
} from '@/lib/api/favorites'
import { create } from 'zustand'

type FavoritesStore = {
  favoritePageIds: Set<string>
  loaded: boolean
  loadFavorites: () => Promise<void>
  addFavorite: (pageId: string) => Promise<void>
  removeFavorite: (pageId: string) => Promise<void>
  clearFavorites: () => void
}

export const useFavoritesStore = create<FavoritesStore>()((set, get) => ({
  favoritePageIds: new Set(),
  loaded: false,
  loadFavorites: async () => {
    try {
      const pages = await getFavorites()
      set({ favoritePageIds: new Set(pages.map((p) => p.id)), loaded: true })
    } catch (err) {
      console.warn('Failed to load favorites:', err)
    }
  },
  addFavorite: async (pageId: string) => {
    const previous = get().favoritePageIds
    const optimistic = new Set(previous)
    optimistic.add(pageId)
    set({ favoritePageIds: optimistic })
    try {
      await addFavoriteAPI(pageId)
    } catch (err) {
      set({ favoritePageIds: previous })
      throw err
    }
  },
  removeFavorite: async (pageId: string) => {
    const previous = get().favoritePageIds
    const optimistic = new Set(previous)
    optimistic.delete(pageId)
    set({ favoritePageIds: optimistic })
    try {
      await removeFavoriteAPI(pageId)
    } catch (err) {
      set({ favoritePageIds: previous })
      throw err
    }
  },
  clearFavorites: () => set({ favoritePageIds: new Set(), loaded: false }),
}))
