import { act, fireEvent, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import MarkdownInlineCode from './MarkdownInlineCode'

const { copyMock } = vi.hoisted(() => ({ copyMock: vi.fn() }))

let mockIsMobile = false

vi.mock('copy-to-clipboard', () => ({ default: copyMock }))
vi.mock('@/lib/useIsMobile', () => ({
  useIsMobile: () => mockIsMobile,
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) =>
      ({
        'codeBlock.copiedAriaLabel': 'Code copied',
        'codeBlock.copiedToast': 'Code copied',
        'codeBlock.copiedTooltip': 'Copied',
        'codeBlock.copyAriaLabel': 'Copy code',
        'codeBlock.copyErrorToast': 'Could not copy code',
        'codeBlock.copyTooltip': 'Copy code',
      })[key] ?? key,
  }),
}))
vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}))
vi.mock('@/components/TooltipWrapper', () => ({
  TooltipWrapper: ({
    children,
    label,
    asChild,
  }: {
    children: ReactNode
    label: string
    asChild?: boolean
  }) => (
    <span
      data-testid="inline-code-tooltip"
      data-label={label}
      data-as-child={String(asChild)}
    >
      {children}
    </span>
  ),
}))

describe('MarkdownInlineCode', () => {
  beforeEach(() => {
    mockIsMobile = false
    copyMock.mockReset()
    copyMock.mockReturnValue(true)
    vi.mocked(toast.error).mockReset()
    vi.mocked(toast.success).mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('copies only the rendered code text and shows temporary feedback', () => {
    vi.useFakeTimers()
    render(
      <MarkdownInlineCode>
        npm <strong>run</strong> test
      </MarkdownInlineCode>,
    )

    const button = screen.getByTestId('markdown-inline-code-copy-button')
    const tooltip = screen.getByTestId('inline-code-tooltip')
    expect(tooltip).toHaveAttribute('data-label', 'Copy code')
    expect(tooltip).toHaveAttribute('data-as-child', 'true')
    fireEvent.click(button)

    expect(copyMock).toHaveBeenCalledWith('npm run test')
    expect(toast.success).toHaveBeenCalled()
    expect(button).toHaveAttribute('aria-label', 'Code copied')
    expect(tooltip).toHaveAttribute('data-label', 'Copied')

    act(() => {
      vi.advanceTimersByTime(2000)
    })
    expect(button).toHaveAttribute('aria-label', 'Copy code')
    expect(tooltip).toHaveAttribute('data-label', 'Copy code')
  })

  it('reports clipboard failures without showing copied feedback', () => {
    copyMock.mockReturnValue(false)
    render(<MarkdownInlineCode>npm test</MarkdownInlineCode>)

    const button = screen.getByTestId('markdown-inline-code-copy-button')
    fireEvent.click(button)

    expect(toast.error).toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
    expect(button).toHaveAttribute('aria-label', 'Copy code')
  })

  it('does not trigger surrounding click handlers', () => {
    const onParentClick = vi.fn()
    render(
      <a href="/target" onClick={onParentClick}>
        <MarkdownInlineCode>npm test</MarkdownInlineCode>
      </a>,
    )

    const button = screen.getByTestId('markdown-inline-code-copy-button')
    expect(fireEvent.click(button)).toBe(false)
    expect(onParentClick).not.toHaveBeenCalled()
    expect(copyMock).toHaveBeenCalledWith('npm test')
  })

  it('does not render the copy button on mobile viewports', () => {
    mockIsMobile = true
    render(<MarkdownInlineCode>npm test</MarkdownInlineCode>)

    expect(
      screen.queryByTestId('markdown-inline-code-copy-button'),
    ).not.toBeInTheDocument()
  })
})
