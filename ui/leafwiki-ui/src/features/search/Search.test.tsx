import { HotKeyHandler } from '@/components/HotKeyHandler'
import { searchPages } from '@/lib/api/search'
import { useHotKeysStore } from '@/stores/hotkeys'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import Search from './Search'

vi.mock('@/lib/api/search', () => ({
  searchPages: vi.fn(),
}))

vi.mock('@/lib/api/tags', () => ({
  fetchTags: vi.fn(() => Promise.resolve([])),
}))

const searchPagesMock = vi.mocked(searchPages)

// Longer than the search input's debounce, so a request that is going to
// be sent has been sent by the time the assertions run.
const PAST_DEBOUNCE_MS = 400

function renderSearch(initialEntry = '/') {
  const ui = (searchActive: boolean) => (
    <MemoryRouter initialEntries={[initialEntry]}>
      <HotKeyHandler />
      <Search active={searchActive} />
    </MemoryRouter>
  )
  const view = render(ui(true))
  return {
    ...view,
    setActive: (searchActive: boolean) => view.rerender(ui(searchActive)),
  }
}

// The history page registers Escape (history.page.close) before the sidebar
// with the search panel is used, and the sidebar stays visible there.
function renderOnHistoryPage() {
  const exitHistory = vi.fn()
  useHotKeysStore.getState().registerHotkey({
    keyCombo: 'Escape',
    enabled: true,
    mode: ['history'],
    action: exitHistory,
  })
  return { exitHistory, ...renderSearch('/history/docs/page') }
}

async function typeQuery(query: string) {
  fireEvent.change(screen.getByTestId('search-input'), {
    target: { value: query },
  })
  await act(async () => {
    vi.advanceTimersByTime(PAST_DEBOUNCE_MS)
  })
}

describe('Search', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    // jsdom has no layout; the result list scrolls the active card into view.
    Element.prototype.scrollIntoView = vi.fn()
    searchPagesMock.mockReset()
    useHotKeysStore.setState({ registeredHotkeys: {} })
    searchPagesMock.mockResolvedValue({
      count: 0,
      items: [],
      limit: 10,
      offset: 0,
      tag_facets: [],
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it.each([
    { name: 'two-letter word', query: 'Pi' },
    { name: 'two characters with a symbol', query: 'C#' },
    { name: 'longer word', query: 'install' },
  ])('sends a search request for a $name', async ({ query }) => {
    renderSearch()

    await typeQuery(query)

    expect(searchPagesMock).toHaveBeenCalledWith(query, 0, 10, [])
    expect(screen.queryByTestId('search-min-length-hint')).toBeNull()
  })

  it.each([
    { name: 'a single character', query: 'a' },
    { name: 'a single character surrounded by spaces', query: ' a ' },
  ])(
    'shows the minimum length hint and sends no request for $name',
    async ({ query }) => {
      renderSearch()

      await typeQuery(query)

      expect(searchPagesMock).not.toHaveBeenCalled()
      expect(screen.getByTestId('search-min-length-hint')).toHaveTextContent(
        'Type at least 2 characters',
      )
      expect(screen.queryByText('No results found.')).toBeNull()
    },
  )

  it('replaces the previous results with the hint when the query gets too short', async () => {
    searchPagesMock.mockResolvedValue({
      count: 1,
      items: [
        {
          page_id: 'p1',
          path: 'docs/pi',
          title: 'Raspberry Pi',
          kind: 'page',
          excerpt: '',
          rank: 1,
          tags: [],
        },
      ],
      limit: 10,
      offset: 0,
      tag_facets: [],
    })
    renderSearch()

    await typeQuery('Pi')
    expect(screen.getByTestId('search-results-list')).toBeInTheDocument()

    await typeQuery('P')

    expect(screen.queryByTestId('search-results-list')).toBeNull()
    expect(screen.getByTestId('search-min-length-hint')).toBeInTheDocument()
    expect(searchPagesMock).toHaveBeenCalledTimes(1)
  })

  it('announces the hint in a status region that exists before the hint', async () => {
    renderSearch()
    const status = screen.getByRole('status')
    expect(status).toBeEmptyDOMElement()

    await typeQuery('a')

    expect(screen.getByRole('status')).toBe(status)
    expect(status).toHaveTextContent('Type at least 2 characters')
  })

  it('debounces fast typing into a single request', async () => {
    renderSearch()
    const input = screen.getByTestId('search-input')

    for (const value of ['Pi', 'Pin', 'Ping']) {
      fireEvent.change(input, { target: { value } })
      await act(async () => {
        vi.advanceTimersByTime(100)
      })
    }
    await act(async () => {
      vi.advanceTimersByTime(PAST_DEBOUNCE_MS)
    })

    expect(searchPagesMock).toHaveBeenCalledTimes(1)
    expect(searchPagesMock).toHaveBeenCalledWith('Ping', 0, 10, [])
  })

  it('keeps filtering by tag and shows the hint when a tag is combined with a too-short query', async () => {
    renderSearch('/?tags=howto')

    await typeQuery('a')

    expect(screen.getByTestId('search-min-length-hint')).toBeInTheDocument()
    expect(searchPagesMock).toHaveBeenLastCalledWith('', 0, 10, ['howto'])
  })

  it('shows no hint for an empty query', async () => {
    renderSearch()

    await typeQuery('')

    expect(screen.queryByTestId('search-min-length-hint')).toBeNull()
    expect(searchPagesMock).not.toHaveBeenCalled()
  })

  describe('syntax help', () => {
    const examples = [
      'install',
      'wiki backup',
      '"exact phrase"',
      'title:word',
      'docker OR podman',
      'wiki NOT draft',
      'instal*',
    ]

    const getHelpButton = () =>
      screen.getByRole('button', { name: 'Search syntax help' })
    const getHelpPanel = () => screen.getByTestId('search-syntax-help')

    it('is collapsed by default', () => {
      renderSearch()

      expect(getHelpButton()).toHaveAttribute('aria-expanded', 'false')
      expect(getHelpPanel()).not.toBeVisible()
    })

    it('opens on a click, lists every example and closes on a second click', () => {
      renderSearch()
      const helpButton = getHelpButton()

      fireEvent.click(helpButton)

      expect(helpButton).toHaveAttribute('aria-expanded', 'true')
      expect(helpButton).toHaveAttribute('aria-controls', getHelpPanel().id)
      expect(getHelpPanel()).toBeVisible()
      for (const example of examples) {
        expect(getHelpPanel()).toHaveTextContent(example)
      }

      fireEvent.click(helpButton)

      expect(helpButton).toHaveAttribute('aria-expanded', 'false')
      expect(getHelpPanel()).not.toBeVisible()
    })

    it.each([
      { name: 'the help button', target: getHelpButton },
      { name: 'the help panel', target: getHelpPanel },
    ])('closes on Escape while focus is on $name', ({ target }) => {
      renderSearch()
      fireEvent.click(getHelpButton())

      fireEvent.keyDown(target(), { key: 'Escape' })

      expect(getHelpButton()).toHaveAttribute('aria-expanded', 'false')
      expect(getHelpPanel()).not.toBeVisible()
    })

    it('closes on Escape while focus is outside the search panel', () => {
      renderSearch()
      fireEvent.click(getHelpButton())

      fireEvent.keyDown(document.body, { key: 'Escape' })

      expect(getHelpPanel()).not.toBeVisible()
      expect(getHelpButton()).toHaveFocus()
    })

    it.each([
      {
        name: 'Escape',
        close: () => fireEvent.keyDown(getHelpPanel(), { key: 'Escape' }),
      },
      { name: 'a second click', close: () => fireEvent.click(getHelpButton()) },
    ])(
      'hands Escape back to the page once the panel is closed by $name',
      ({ close }) => {
        const { exitHistory } = renderOnHistoryPage()
        fireEvent.click(getHelpButton())
        close()

        fireEvent.keyDown(document.body, { key: 'Escape' })

        expect(exitHistory).toHaveBeenCalledTimes(1)
      },
    )

    it('leaves Escape to the page while the sidebar shows another panel', () => {
      const { exitHistory, setActive } = renderOnHistoryPage()
      fireEvent.click(getHelpButton())

      setActive(false)
      fireEvent.keyDown(document.body, { key: 'Escape' })

      expect(exitHistory).toHaveBeenCalledTimes(1)
      expect(getHelpButton()).toHaveAttribute('aria-expanded', 'true')
    })

    it('removes its Escape hotkey when unmounted while open', () => {
      const { exitHistory, unmount } = renderOnHistoryPage()
      fireEvent.click(getHelpButton())

      unmount()

      expect(useHotKeysStore.getState().registeredHotkeys.Escape).toEqual([
        expect.objectContaining({ action: exitHistory }),
      ])
    })

    it('stays open while typing a query', async () => {
      renderSearch()
      fireEvent.click(getHelpButton())

      await typeQuery('install')

      expect(getHelpButton()).toHaveAttribute('aria-expanded', 'true')
      expect(getHelpPanel()).toBeVisible()
    })
  })
})
