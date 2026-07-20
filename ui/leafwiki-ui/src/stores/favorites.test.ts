import type { Page } from '@/lib/api/pages'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/api/favorites', () => ({
  getFavorites: vi.fn(),
  addFavorite: vi.fn(),
  removeFavorite: vi.fn(),
}))

import * as favoritesAPI from '@/lib/api/favorites'
import { useFavoritesStore } from './favorites'

function makePage(id: string): Page {
  return {
    id,
    slug: id,
    path: id,
    title: id,
    content: '',
    version: 'v1',
    kind: 'page',
  }
}

describe('useFavoritesStore', () => {
  beforeEach(() => {
    useFavoritesStore.setState({ favoritePageIds: new Set(), loaded: false })
    vi.mocked(favoritesAPI.getFavorites).mockReset()
    vi.mocked(favoritesAPI.addFavorite).mockReset()
    vi.mocked(favoritesAPI.removeFavorite).mockReset()
  })

  it('loadFavorites populates favoritePageIds from the API', async () => {
    vi.mocked(favoritesAPI.getFavorites).mockResolvedValue([
      makePage('page-1'),
      makePage('page-2'),
    ])

    await useFavoritesStore.getState().loadFavorites()

    expect(useFavoritesStore.getState().favoritePageIds).toEqual(
      new Set(['page-1', 'page-2']),
    )
    expect(useFavoritesStore.getState().loaded).toBe(true)
  })

  it('loadFavorites swallows API errors and leaves state as-is', async () => {
    vi.mocked(favoritesAPI.getFavorites).mockRejectedValue(
      new Error('network error'),
    )

    await expect(
      useFavoritesStore.getState().loadFavorites(),
    ).resolves.toBeUndefined()
    expect(useFavoritesStore.getState().favoritePageIds.size).toBe(0)
  })

  it('addFavorite optimistically adds the page id before the API resolves', async () => {
    let resolveAdd: () => void = () => {}
    vi.mocked(favoritesAPI.addFavorite).mockReturnValue(
      new Promise((resolve) => {
        resolveAdd = () => resolve(undefined)
      }),
    )

    const promise = useFavoritesStore.getState().addFavorite('page-1')
    expect(useFavoritesStore.getState().favoritePageIds.has('page-1')).toBe(
      true,
    )

    resolveAdd()
    await promise
    expect(useFavoritesStore.getState().favoritePageIds.has('page-1')).toBe(
      true,
    )
  })

  it('addFavorite rolls back the optimistic update when the API call fails', async () => {
    vi.mocked(favoritesAPI.addFavorite).mockRejectedValue(new Error('boom'))

    await expect(
      useFavoritesStore.getState().addFavorite('page-1'),
    ).rejects.toThrow('boom')
    expect(useFavoritesStore.getState().favoritePageIds.has('page-1')).toBe(
      false,
    )
  })

  it('removeFavorite optimistically removes the page id before the API resolves', async () => {
    useFavoritesStore.setState({ favoritePageIds: new Set(['page-1']) })
    let resolveRemove: () => void = () => {}
    vi.mocked(favoritesAPI.removeFavorite).mockReturnValue(
      new Promise((resolve) => {
        resolveRemove = () => resolve(undefined)
      }),
    )

    const promise = useFavoritesStore.getState().removeFavorite('page-1')
    expect(useFavoritesStore.getState().favoritePageIds.has('page-1')).toBe(
      false,
    )

    resolveRemove()
    await promise
    expect(useFavoritesStore.getState().favoritePageIds.has('page-1')).toBe(
      false,
    )
  })

  it('removeFavorite rolls back the optimistic update when the API call fails', async () => {
    useFavoritesStore.setState({ favoritePageIds: new Set(['page-1']) })
    vi.mocked(favoritesAPI.removeFavorite).mockRejectedValue(new Error('boom'))

    await expect(
      useFavoritesStore.getState().removeFavorite('page-1'),
    ).rejects.toThrow('boom')
    expect(useFavoritesStore.getState().favoritePageIds.has('page-1')).toBe(
      true,
    )
  })

  it('clearFavorites resets to empty and not-loaded', () => {
    useFavoritesStore.setState({
      favoritePageIds: new Set(['page-1']),
      loaded: true,
    })

    useFavoritesStore.getState().clearFavorites()

    expect(useFavoritesStore.getState().favoritePageIds.size).toBe(0)
    expect(useFavoritesStore.getState().loaded).toBe(false)
  })
})
