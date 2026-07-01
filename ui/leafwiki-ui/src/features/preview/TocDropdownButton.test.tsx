import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TocDropdownButton } from './TocDropdownButton'
import type { TocEntry } from './extractTocEntries'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'toc.onThisPage': 'On this page',
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
  scrollContainer = document.createElement('div')
  scrollContainer.id = 'scroll-container'
  vi.spyOn(scrollContainer, 'getBoundingClientRect').mockReturnValue({
    top: 0,
  } as DOMRect)
  document.body.appendChild(scrollContainer)
})

afterEach(() => {
  // Remove manually added DOM elements; React component cleanup is handled
  // automatically by @testing-library/react — setting innerHTML='' directly
  // would break Radix portal teardown.
  scrollContainer?.remove()
  document.querySelectorAll('h2[id]').forEach((el) => el.remove())
})

async function openDropdown() {
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: /on this page/i }))
}

describe('TocDropdownButton — rendering', () => {
  it('renders the trigger button', () => {
    render(<TocDropdownButton entries={entries} />)
    expect(
      screen.getByRole('button', { name: /on this page/i }),
    ).toBeInTheDocument()
  })

  it('shows all entries after opening', async () => {
    render(<TocDropdownButton entries={entries} />)
    await openDropdown()
    expect(await screen.findByText('Introduction')).toBeInTheDocument()
    expect(screen.getByText('Background')).toBeInTheDocument()
    expect(screen.getByText('Details')).toBeInTheDocument()
  })
})

describe('TocDropdownButton — active state', () => {
  it('applies text-brand to the active entry', async () => {
    addHeading('introduction', 50) // 50 <= 120 → active
    addHeading('background', 300)
    addHeading('details', 600)

    render(<TocDropdownButton entries={entries} />)
    await openDropdown()

    const introItem = await screen.findByText('Introduction')
    expect(introItem.closest('[role="menuitem"]')?.className).toMatch(
      /text-brand/,
    )
  })

  it('updates active entry on scroll', async () => {
    const introEl = addHeading('introduction', 50)
    const bgEl = addHeading('background', 300)
    addHeading('details', 600)

    render(<TocDropdownButton entries={entries} />)

    vi.spyOn(introEl, 'getBoundingClientRect').mockReturnValue({
      top: -100,
    } as DOMRect)
    vi.spyOn(bgEl, 'getBoundingClientRect').mockReturnValue({
      top: 50,
    } as DOMRect)

    act(() => {
      scrollContainer.dispatchEvent(new Event('scroll'))
    })

    await openDropdown()

    await waitFor(() => {
      const bgItem = screen.getByText('Background')
      expect(bgItem.closest('[role="menuitem"]')?.className).toMatch(
        /text-brand/,
      )
    })
  })

  it('does not apply text-brand to inactive entries', async () => {
    addHeading('introduction', 50)
    addHeading('background', 300)
    addHeading('details', 600)

    render(<TocDropdownButton entries={entries} />)
    await openDropdown()

    const bgItem = await screen.findByText('Background')
    expect(bgItem.closest('[role="menuitem"]')?.className).not.toMatch(
      /text-brand/,
    )
  })
})

describe('TocDropdownButton — click navigation', () => {
  it('calls scrollToHeadlineHash when entry is clicked', async () => {
    render(<TocDropdownButton entries={entries} clickable />)
    await openDropdown()
    const user = userEvent.setup()
    await user.click(await screen.findByText('Background'))
    expect(scrollMock).toHaveBeenCalledWith('#background', {
      waitForStableLayout: false,
    })
  })

  it('does not call scrollToHeadlineHash when not clickable', async () => {
    render(<TocDropdownButton entries={entries} clickable={false} />)
    await openDropdown()
    const user = userEvent.setup()
    await user.click(await screen.findByText('Background'))
    expect(scrollMock).not.toHaveBeenCalled()
  })
})
