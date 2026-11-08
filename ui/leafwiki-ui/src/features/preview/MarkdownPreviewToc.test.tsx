import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TooltipProvider } from '@/components/ui/tooltip'
import MarkdownPreview from './MarkdownPreview'

// The inline TOC dropdown MarkdownPreview renders above the article when it
// gets showToc and more than three headings.

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => (key === 'toc.onThisPage' ? 'On this page' : key),
    i18n: { language: 'en' },
  }),
}))

const scrollMock = vi.fn()
vi.mock('@/lib/scrollToHeadline', () => ({
  scrollToHeadlineHash: (...args: unknown[]) => scrollMock(...args),
}))

let intersect: (isIntersecting: boolean) => void = () => {}

class FakeIntersectionObserver {
  constructor(cb: (entries: { isIntersecting: boolean }[]) => void) {
    intersect = (isIntersecting) => cb([{ isIntersecting }])
  }
  observe() {}
  disconnect() {}
}

const fourHeadings = '# One\n\n## Two\n\n## Three\n\n## Four\n'

let scrollContainer: HTMLDivElement

beforeEach(() => {
  vi.clearAllMocks()
  vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }))
  scrollContainer = document.createElement('div')
  scrollContainer.id = 'scroll-container'
  document.body.appendChild(scrollContainer)
})

afterEach(() => {
  scrollContainer.remove()
  vi.unstubAllGlobals()
})

function renderPreview(props: {
  content: string
  showToc?: boolean
  tocClickable?: boolean
}) {
  return render(
    <TooltipProvider>
      <MarkdownPreview {...props} />
    </TooltipProvider>,
  )
}

describe('MarkdownPreview — inline TOC dropdown', () => {
  it.each([
    { name: 'showToc off', showToc: false, content: fourHeadings },
    {
      name: 'three headings',
      showToc: true,
      content: '# One\n\n## Two\n\n## Three\n',
    },
  ])('renders no dropdown with $name', ({ showToc, content }) => {
    renderPreview({ content, showToc })
    expect(
      screen.queryByRole('button', { name: 'On this page' }),
    ).not.toBeInTheDocument()
  })

  it.each([
    { name: 'clickable', tocClickable: true, scrolls: true },
    { name: 'not clickable', tocClickable: false, scrolls: false },
  ])(
    'lists the headings and scrolls on select only when $name',
    async ({ tocClickable, scrolls }) => {
      renderPreview({ content: fourHeadings, showToc: true, tocClickable })
      const user = userEvent.setup()
      await user.click(screen.getByRole('button', { name: 'On this page' }))
      await user.click(await screen.findByTestId('toc-dropdown-entry-three'))
      expect(scrollMock).toHaveBeenCalledTimes(scrolls ? 1 : 0)
    },
  )

  it('runs the scroll spy only while the inline dropdown is shown', () => {
    const add = vi.spyOn(scrollContainer, 'addEventListener')
    const remove = vi.spyOn(scrollContainer, 'removeEventListener')
    renderPreview({ content: fourHeadings, showToc: true })
    expect(add).toHaveBeenCalledWith('scroll', expect.any(Function), {
      passive: true,
    })

    // Scrolled past the dropdown: it unmounts (the sticky TOC takes over).
    act(() => intersect(false))
    expect(
      screen.queryByRole('button', { name: 'On this page' }),
    ).not.toBeInTheDocument()
    expect(remove).toHaveBeenCalledWith('scroll', expect.any(Function))

    act(() => intersect(true))
    expect(
      screen.getByRole('button', { name: 'On this page' }),
    ).toBeInTheDocument()
    expect(add).toHaveBeenCalledTimes(2)
  })
})
