import { act, render, screen, fireEvent } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useTocPanelStore } from '@/stores/tocPanel'
import { TocSidePanel } from './TocSidePanel'
import type { TocEntry } from './extractTocEntries'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'toc.title': 'Table of contents',
        'toc.onThisPage': 'On this page',
        'toc.collapse': 'Collapse table of contents',
        'toc.expand': 'Expand table of contents',
      }
      return map[key] ?? key
    },
  }),
}))

const scrollMock = vi.fn()
vi.mock('@/lib/scrollToHeadline', () => ({
  scrollToHeadlineHash: (...args: unknown[]) => scrollMock(...args),
}))

const entries: TocEntry[] = [
  { level: 1, text: 'Introduction', id: 'introduction' },
  { level: 2, text: 'Background', id: 'background' },
  { level: 3, text: 'Details', id: 'details' },
]

// Helpers to add heading elements with controlled rect positions
function addHeading(id: string, top: number): HTMLElement {
  const el = document.createElement('h2')
  el.id = id
  vi.spyOn(el, 'getBoundingClientRect').mockReturnValue({ top } as DOMRect)
  document.body.appendChild(el)
  return el
}

let scrollContainer: HTMLDivElement

beforeEach(() => {
  vi.clearAllMocks()
  useTocPanelStore.setState({ collapsed: false })

  scrollContainer = document.createElement('div')
  scrollContainer.id = 'scroll-container'
  // Container top at y=0 → triggerY = 120
  vi.spyOn(scrollContainer, 'getBoundingClientRect').mockReturnValue({
    top: 0,
  } as DOMRect)
  document.body.appendChild(scrollContainer)
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('TocSidePanel — rendering', () => {
  it('renders the "On this page" title', () => {
    render(<TocSidePanel entries={entries} />)
    expect(screen.getByText('On this page')).toBeInTheDocument()
  })

  it('renders all entry texts', () => {
    render(<TocSidePanel entries={entries} />)
    expect(screen.getByText('Introduction')).toBeInTheDocument()
    expect(screen.getByText('Background')).toBeInTheDocument()
    expect(screen.getByText('Details')).toBeInTheDocument()
  })

  it('renders data-testid on each entry button', () => {
    render(<TocSidePanel entries={entries} />)
    expect(screen.getByTestId('toc-entry-introduction')).toBeInTheDocument()
    expect(screen.getByTestId('toc-entry-background')).toBeInTheDocument()
    expect(screen.getByTestId('toc-entry-details')).toBeInTheDocument()
  })

  it('has nav with correct aria-label', () => {
    render(<TocSidePanel entries={entries} />)
    expect(screen.getByRole('navigation')).toHaveAttribute(
      'aria-label',
      'Table of contents',
    )
  })

  it('does not register scroll listener when entries is empty', () => {
    const spy = vi.spyOn(scrollContainer, 'addEventListener')
    render(<TocSidePanel entries={[]} />)
    expect(spy).not.toHaveBeenCalled()
  })
})

describe('TocSidePanel — click navigation', () => {
  it('calls scrollToHeadlineHash with encoded id when entry is clicked', () => {
    render(<TocSidePanel entries={entries} />)
    fireEvent.click(screen.getByTestId('toc-entry-background'))
    expect(scrollMock).toHaveBeenCalledWith('#background', {
      waitForStableLayout: false,
    })
  })

  it('encodes special characters in heading ids', () => {
    const specialEntries: TocEntry[] = [
      { level: 1, text: 'Hello World', id: 'hello world' },
    ]
    render(<TocSidePanel entries={specialEntries} />)
    fireEvent.click(screen.getByTestId('toc-entry-hello world'))
    expect(scrollMock).toHaveBeenCalledWith(
      `#${encodeURIComponent('hello world')}`,
      { waitForStableLayout: false },
    )
  })
})

describe('TocSidePanel — scroll spy', () => {
  it('defaults to first entry as active on mount', () => {
    // All headings below triggerY (0 + 120 = 120)
    entries.forEach(({ id }) => addHeading(id, 300))

    render(<TocSidePanel entries={entries} />)

    expect(screen.getByTestId('toc-entry-introduction').className).toMatch(
      /page-viewer__toc-panel-entry--active/,
    )
  })

  it('activates heading whose top has passed triggerY on mount', () => {
    addHeading('introduction', 50) // 50 <= 120 → active
    addHeading('background', 300)
    addHeading('details', 500)

    render(<TocSidePanel entries={entries} />)

    expect(screen.getByTestId('toc-entry-introduction').className).toMatch(
      /page-viewer__toc-panel-entry--active/,
    )
  })

  it('updates active heading when scrolling', () => {
    const introEl = addHeading('introduction', 50) // already past trigger
    const bgEl = addHeading('background', 300)
    addHeading('details', 600)

    render(<TocSidePanel entries={entries} />)

    // Scroll: background now also past trigger
    vi.spyOn(introEl, 'getBoundingClientRect').mockReturnValue({
      top: -100,
    } as DOMRect)
    vi.spyOn(bgEl, 'getBoundingClientRect').mockReturnValue({
      top: 50,
    } as DOMRect)

    act(() => {
      scrollContainer.dispatchEvent(new Event('scroll'))
    })

    expect(screen.getByTestId('toc-entry-background').className).toMatch(
      /page-viewer__toc-panel-entry--active/,
    )
  })

  it('picks the deepest heading past triggerY (last one in document order)', () => {
    addHeading('introduction', -200)
    addHeading('background', 50) // both past trigger → background wins
    addHeading('details', 300)

    render(<TocSidePanel entries={entries} />)

    expect(screen.getByTestId('toc-entry-background').className).toMatch(
      /page-viewer__toc-panel-entry--active/,
    )
    expect(screen.getByTestId('toc-entry-introduction').className).not.toMatch(
      /page-viewer__toc-panel-entry--active/,
    )
  })

  it('activates last visible heading when scroll is at the bottom', () => {
    // scrollContainer at bottom: scrollHeight - scrollTop - clientHeight < 5
    Object.defineProperty(scrollContainer, 'scrollHeight', {
      value: 1000,
      configurable: true,
    })
    Object.defineProperty(scrollContainer, 'scrollTop', {
      value: 900,
      configurable: true,
    })
    Object.defineProperty(scrollContainer, 'clientHeight', {
      value: 100,
      configurable: true,
    })
    // containerRect: top=0, bottom=100
    vi.spyOn(scrollContainer, 'getBoundingClientRect').mockReturnValue({
      top: 0,
      bottom: 100,
    } as DOMRect)

    // All headings are BELOW triggerY (120) but still inside the viewport (< bottom 100)
    // introduction: top=200 — not visible, not past trigger
    // background:   top=40  — below trigger(120) but inside viewport(100)? no: 40<100 → visible
    // details:      top=70  — also inside viewport
    addHeading('introduction', 200)
    addHeading('background', 40) // 40 < bottom(100) → visible at bottom → active
    addHeading('details', 70) // 70 < bottom(100) → visible, wins as last

    render(<TocSidePanel entries={entries} />)

    expect(screen.getByTestId('toc-entry-details').className).toMatch(
      /page-viewer__toc-panel-entry--active/,
    )
  })

  it('removes scroll listener on unmount', () => {
    entries.forEach(({ id }) => addHeading(id, 300))
    const removeSpy = vi.spyOn(scrollContainer, 'removeEventListener')
    const { unmount } = render(<TocSidePanel entries={entries} />)
    unmount()
    expect(removeSpy).toHaveBeenCalledWith('scroll', expect.any(Function))
  })
})

describe('TocSidePanel — collapse/expand', () => {
  it('collapses the panel when the collapse button is clicked', () => {
    render(<TocSidePanel entries={entries} />)

    // Single, always-mounted toggle button — its testid/label swap with state
    // instead of a separate collapse/expand element, so its position never
    // changes.
    fireEvent.click(screen.getByTestId('toc-side-panel-collapse'))

    expect(screen.getByTestId('toc-side-panel')).toBeInTheDocument()
    expect(screen.getByTestId('toc-side-panel-list')).toHaveAttribute(
      'aria-hidden',
      'true',
    )
    expect(screen.getByTestId('toc-side-panel-expand')).toBeInTheDocument()
    expect(
      screen.queryByTestId('toc-side-panel-collapse'),
    ).not.toBeInTheDocument()
    expect(useTocPanelStore.getState().collapsed).toBe(true)
  })

  it('expands the panel again when the expand button is clicked', () => {
    useTocPanelStore.setState({ collapsed: true })
    render(<TocSidePanel entries={entries} />)

    fireEvent.click(screen.getByTestId('toc-side-panel-expand'))

    expect(screen.getByTestId('toc-side-panel-list')).toHaveAttribute(
      'aria-hidden',
      'false',
    )
    expect(screen.getByTestId('toc-side-panel-collapse')).toBeInTheDocument()
    expect(
      screen.queryByTestId('toc-side-panel-expand'),
    ).not.toBeInTheDocument()
    expect(useTocPanelStore.getState().collapsed).toBe(false)
  })

  it('renders collapsed when the store starts out collapsed', () => {
    useTocPanelStore.setState({ collapsed: true })
    render(<TocSidePanel entries={entries} />)

    expect(screen.getByTestId('toc-side-panel-expand')).toBeInTheDocument()
    expect(screen.getByTestId('toc-side-panel-list')).toHaveAttribute(
      'aria-hidden',
      'true',
    )
  })

  it('toggles the --collapsed modifier class on the nav element', () => {
    render(<TocSidePanel entries={entries} />)

    // Drives the border-color fade here; AppLayout applies the matching
    // modifier to .app-layout__toc-pane to shrink its width (see index.css).
    expect(screen.getByTestId('toc-side-panel').className).not.toMatch(
      /page-viewer__toc-panel--collapsed/,
    )

    fireEvent.click(screen.getByTestId('toc-side-panel-collapse'))

    expect(screen.getByTestId('toc-side-panel').className).toMatch(
      /page-viewer__toc-panel--collapsed/,
    )
  })
})
