import { HotKeyHandler } from '@/components/HotKeyHandler'
import { ApiLocalizedError } from '@/lib/api/errors'
import { searchPages } from '@/lib/api/search'
import { useHotKeysStore } from '@/stores/hotkeys'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  onTestFinished,
  vi,
} from 'vitest'
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

const twoResults = {
  count: 2,
  items: [
    {
      page_id: 'p1',
      path: 'docs/pi',
      title: 'Raspberry Pi',
      kind: 'page' as const,
      excerpt: '',
      rank: 1,
      tags: [],
    },
    {
      page_id: 'p2',
      path: 'docs/pin',
      title: 'Pinning',
      kind: 'page' as const,
      excerpt: '',
      rank: 0.5,
      tags: [],
    },
  ],
  limit: 10,
  offset: 0,
  tag_facets: [],
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
    { name: 'two non-ASCII letters', query: 'üb' },
    { name: 'two CJK characters', query: '搜索' },
    { name: 'pasted long query', query: 'backup '.repeat(200).trim() },
  ])('sends a search request for a $name', async ({ query }) => {
    renderSearch()

    await typeQuery(query)

    expect(searchPagesMock).toHaveBeenCalledWith(query, 0, 10, [])
    expect(screen.queryByTestId('search-min-length-hint')).toBeNull()
  })

  it.each([
    { name: 'a single character', query: 'a' },
    { name: 'a single character surrounded by spaces', query: ' a ' },
    { name: 'a single non-ASCII letter', query: 'ü' },
    { name: 'a single CJK character', query: '中' },
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

  it.each([
    { name: 'an empty query', query: '' },
    { name: 'a whitespace-only query', query: '   ' },
  ])('shows no hint for $name', async ({ query }) => {
    renderSearch()

    await typeQuery(query)

    expect(screen.queryByTestId('search-min-length-hint')).toBeNull()
    expect(searchPagesMock).not.toHaveBeenCalled()
  })

  it.each([
    { name: 'a one-character', url: '/?q=a', sent: false },
    { name: 'a two-character', url: '/?q=ab', sent: true },
  ])('handles $name query in a deep link', async ({ url, sent }) => {
    renderSearch(url)
    await act(async () => {
      vi.advanceTimersByTime(PAST_DEBOUNCE_MS)
    })

    expect(screen.getByTestId('search-input')).toHaveValue(
      new URLSearchParams(url.slice(1)).get('q'),
    )
    if (sent) {
      expect(searchPagesMock).toHaveBeenCalledWith('ab', 0, 10, [])
      expect(screen.queryByTestId('search-min-length-hint')).toBeNull()
    } else {
      expect(searchPagesMock).not.toHaveBeenCalled()
      expect(screen.getByTestId('search-min-length-hint')).toBeInTheDocument()
      expect(screen.queryByText('Loading results...')).toBeNull()
    }
  })

  it('ignores an in-flight response once the query is cut down below the minimum', async () => {
    let resolveSearch: (value: typeof twoResults) => void = () => {}
    searchPagesMock.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveSearch = resolve
      }),
    )
    renderSearch()

    await typeQuery('Pin')
    expect(searchPagesMock).toHaveBeenCalledTimes(1)
    fireEvent.change(screen.getByTestId('search-input'), {
      target: { value: 'P' },
    })
    await act(async () => {
      resolveSearch(twoResults)
    })

    expect(screen.queryByTestId('search-results-list')).toBeNull()
    expect(screen.getByTestId('search-min-length-hint')).toBeInTheDocument()
  })

  // Regression: the debounced query never changed, so no request settled
  // the loading state.
  it('searches again when a too-short query is typed back within the debounce', async () => {
    searchPagesMock.mockResolvedValue(twoResults)
    renderSearch()
    const input = screen.getByTestId('search-input')

    await typeQuery('Pi')
    expect(screen.getByTestId('search-results-list')).toBeInTheDocument()

    fireEvent.change(input, { target: { value: 'P' } })
    fireEvent.change(input, { target: { value: 'Pi' } })
    await act(async () => {
      vi.advanceTimersByTime(PAST_DEBOUNCE_MS)
    })

    expect(screen.queryByText('Loading results...')).toBeNull()
    expect(screen.getByTestId('search-results-list')).toBeInTheDocument()
  })

  it('moves the selection with ArrowDown from the input', async () => {
    searchPagesMock.mockResolvedValue(twoResults)
    renderSearch()

    await typeQuery('Pi')
    fireEvent.keyDown(screen.getByTestId('search-input'), {
      key: 'ArrowDown',
    })

    expect(screen.getByTestId('search-result-card-p2')).toHaveClass(
      'search-result-card--selected',
    )
    expect(screen.getByTestId('search-result-card-p1')).not.toHaveClass(
      'search-result-card--selected',
    )
  })

  describe('over-long query', () => {
    const tooLong = new ApiLocalizedError({
      code: 'search_query_too_long',
      message: 'Search query is too long: at most 256 characters and 12 terms',
      template: 'search query is too long: at most %s characters and %s terms',
      args: ['256', '12'],
    })
    const tooLongHint =
      'Search query is too long: at most 256 characters and 12 terms'

    it('caps the input at the backend limit of 256 characters', () => {
      renderSearch()

      expect(screen.getByTestId('search-input')).toHaveAttribute(
        'maxLength',
        '256',
      )
    })

    it('shows the localized limit in the status region instead of no results', async () => {
      const consoleError = vi.spyOn(console, 'error')
      onTestFinished(() => consoleError.mockRestore())
      searchPagesMock.mockRejectedValue(tooLong)
      renderSearch()

      await typeQuery('pi '.repeat(13).trim())

      expect(screen.getByRole('status')).toHaveTextContent(tooLongHint)
      expect(screen.queryByText('No results found.')).toBeNull()
      expect(consoleError).not.toHaveBeenCalled()
    })

    it('clears the limit hint when the query changes', async () => {
      searchPagesMock.mockRejectedValueOnce(tooLong)
      renderSearch()

      await typeQuery('pi '.repeat(13).trim())
      expect(screen.getByRole('status')).toHaveTextContent(tooLongHint)

      await typeQuery('pi')

      expect(screen.getByRole('status')).toBeEmptyDOMElement()
      expect(screen.getByText('No results found.')).toBeInTheDocument()
    })

    it('clears the limit hint with the clear button', async () => {
      searchPagesMock.mockRejectedValueOnce(tooLong)
      renderSearch()

      await typeQuery('pi '.repeat(13).trim())
      expect(screen.getByRole('status')).toHaveTextContent(tooLongHint)

      fireEvent.click(screen.getByTestId('search-clear-button'))

      expect(screen.queryByTestId('search-query-limit-hint')).toBeNull()
      expect(screen.getByRole('status')).toBeEmptyDOMElement()
    })

    it('shows the limit hint for an over-long query from the URL', async () => {
      searchPagesMock.mockRejectedValue(tooLong)
      renderSearch(`/?q=${encodeURIComponent('pi '.repeat(13).trim())}`)

      await act(async () => {
        vi.advanceTimersByTime(PAST_DEBOUNCE_MS)
      })

      expect(screen.getByRole('status')).toHaveTextContent(tooLongHint)
      expect(screen.queryByText('No results found.')).toBeNull()
    })

    it('ignores a limit rejection for a query that was replaced meanwhile', async () => {
      let rejectStale: (err: unknown) => void = () => {}
      searchPagesMock
        .mockImplementationOnce(
          () =>
            new Promise((_, reject) => {
              rejectStale = reject
            }),
        )
        .mockResolvedValueOnce(twoResults)
      renderSearch()

      await typeQuery('pi '.repeat(13).trim())
      await typeQuery('pi')
      await act(async () => {
        rejectStale(tooLong)
      })

      expect(screen.queryByTestId('search-query-limit-hint')).toBeNull()
      expect(screen.getByText('Raspberry Pi')).toBeInTheDocument()
    })

    it('still logs and shows no results for other search errors', async () => {
      const consoleError = vi
        .spyOn(console, 'error')
        .mockImplementation(() => {})
      onTestFinished(() => consoleError.mockRestore())
      searchPagesMock.mockRejectedValue(
        new ApiLocalizedError({
          code: 'search_unavailable',
          message: 'Search is currently unavailable',
          template: 'search is currently unavailable',
          args: [],
        }),
      )
      renderSearch()

      await typeQuery('pi')

      expect(screen.queryByTestId('search-query-limit-hint')).toBeNull()
      expect(screen.getByText('No results found.')).toBeInTheDocument()
      expect(consoleError).toHaveBeenCalled()
    })
  })

  // jsdom computes no layout, so this only proves the class survives
  // tailwind-merge against the Input's px-3; whether text clears the button
  // is checked visually.
  it('reserves right padding on the input for the clear button', () => {
    renderSearch()

    expect(screen.getByTestId('search-input')).toHaveClass('pr-8')
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

    it('keeps the query when Escape closes the panel', async () => {
      renderSearch()
      await typeQuery('install')
      fireEvent.click(getHelpButton())

      fireEvent.keyDown(getHelpPanel(), { key: 'Escape' })

      expect(screen.getByTestId('search-input')).toHaveValue('install')
      expect(getHelpButton()).toHaveFocus()
    })

    it('closes on Escape on a page that also exits on Escape, without exiting it', () => {
      const { exitHistory } = renderOnHistoryPage()
      fireEvent.click(getHelpButton())

      fireEvent.keyDown(getHelpPanel(), { key: 'Escape' })

      expect(getHelpPanel()).not.toBeVisible()
      expect(exitHistory).not.toHaveBeenCalled()
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

    it('keeps the page Escape registered after the panel opened when the button closes it', () => {
      renderSearch('/history/docs/page')
      fireEvent.click(getHelpButton())
      // The history page registers its Escape above the open panel, e.g.
      // when navigating there with the panel open.
      const exitHistory = vi.fn()
      useHotKeysStore.getState().registerHotkey({
        keyCombo: 'Escape',
        enabled: true,
        mode: ['history'],
        action: exitHistory,
      })

      fireEvent.click(getHelpButton())
      fireEvent.keyDown(document.body, { key: 'Escape' })

      expect(exitHistory).toHaveBeenCalledTimes(1)
    })

    it('leaves Escape to the page while the sidebar shows another panel', () => {
      const { exitHistory, setActive } = renderOnHistoryPage()
      fireEvent.click(getHelpButton())

      setActive(false)
      fireEvent.keyDown(document.body, { key: 'Escape' })

      expect(exitHistory).toHaveBeenCalledTimes(1)
      expect(getHelpButton()).toHaveAttribute('aria-expanded', 'true')
    })

    it('leaves Escape to CodeMirror while focus is in the editor', () => {
      renderSearch('/e/docs/page')
      const editor = document.createElement('div')
      editor.className = 'cm-editor'
      const content = document.createElement('div')
      content.tabIndex = 0
      editor.appendChild(content)
      document.body.appendChild(editor)
      onTestFinished(() => editor.remove())
      fireEvent.click(getHelpButton())
      content.focus()

      const notPrevented = fireEvent.keyDown(content, { key: 'Escape' })

      expect(notPrevented).toBe(true)
      expect(getHelpPanel()).toBeVisible()
    })

    it('removes its Escape hotkey when unmounted while open', () => {
      const { exitHistory, unmount } = renderOnHistoryPage()
      fireEvent.click(getHelpButton())

      unmount()

      expect(useHotKeysStore.getState().registeredHotkeys.Escape).toEqual([
        expect.objectContaining({ action: exitHistory }),
      ])
    })

    it('ignores Escape on the button while the panel is closed', () => {
      renderSearch()
      const helpButton = getHelpButton()

      fireEvent.keyDown(helpButton, { key: 'Escape' })

      expect(helpButton).toHaveAttribute('aria-expanded', 'false')
    })

    it('comes after the input and clear button and before the panel in focus order', async () => {
      renderSearch()
      await typeQuery('install')
      fireEvent.click(getHelpButton())

      const order = [
        screen.getByTestId('search-input'),
        screen.getByTestId('search-clear-button'),
        getHelpButton(),
        getHelpPanel(),
      ]
      for (let i = 1; i < order.length; i++) {
        expect(
          order[i - 1].compareDocumentPosition(order[i]) &
            Node.DOCUMENT_POSITION_FOLLOWING,
        ).toBeTruthy()
      }
      // The panel is a programmatic focus target, not a tab stop.
      expect(getHelpPanel()).toHaveAttribute('tabindex', '-1')
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
