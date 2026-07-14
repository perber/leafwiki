import type { PageNode } from '@/lib/api/pages'
import { useFavoritesStore } from '@/stores/favorites'
import { useTreeStore } from '@/stores/tree'
import { render, screen } from '@testing-library/react'
import type React from 'react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { FavoritesSection } from './FavoritesSection'

vi.mock('react-i18next', () => ({
  initReactI18next: {
    type: '3rdParty',
    init: () => {},
  },
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@/components/TooltipWrapper', () => ({
  TooltipWrapper: ({ children }: { children: React.ReactNode }) => children,
}))

function makeNode(id: string, title: string): PageNode {
  return {
    id,
    title,
    slug: id,
    path: id,
    version: 'v1',
    children: null,
    kind: 'page',
  }
}

describe('FavoritesSection', () => {
  beforeEach(() => {
    useFavoritesStore.setState({ favoritePageIds: new Set(), loaded: false })
    useTreeStore.setState({ byId: {} })
  })

  it('renders nothing when there are no favorites', () => {
    const { container } = render(
      <MemoryRouter>
        <FavoritesSection />
      </MemoryRouter>,
    )
    expect(container).toBeEmptyDOMElement()
  })

  it('renders favorited pages resolved from the tree store, sorted by title', () => {
    useTreeStore.setState({
      byId: {
        'page-1': makeNode('page-1', 'Zebra'),
        'page-2': makeNode('page-2', 'Alpha'),
      },
    })
    useFavoritesStore.setState({
      favoritePageIds: new Set(['page-1', 'page-2']),
    })

    render(
      <MemoryRouter>
        <FavoritesSection />
      </MemoryRouter>,
    )

    const items = screen.getAllByTestId('favorite-item')
    expect(items).toHaveLength(2)
    expect(items[0]).toHaveTextContent('Alpha')
    expect(items[1]).toHaveTextContent('Zebra')
  })

  it('silently skips a favorited page id that no longer resolves in the tree', () => {
    useTreeStore.setState({
      byId: { 'page-1': makeNode('page-1', 'Still here') },
    })
    useFavoritesStore.setState({
      favoritePageIds: new Set(['page-1', 'stale-id']),
    })

    render(
      <MemoryRouter>
        <FavoritesSection />
      </MemoryRouter>,
    )

    expect(screen.getAllByTestId('favorite-item')).toHaveLength(1)
    expect(screen.getByText('Still here')).toBeInTheDocument()
  })
})
