import '@/lib/i18n'
import { ApiLocalizedError } from '@/lib/api/errors'
import { useConfigStore } from '@/stores/config'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import DisplaySettings from './DisplaySettings'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

import { toast } from 'sonner'

const setAlwaysShowToc = vi.fn()
const setContentWidth = vi.fn()

function setConfig(
  overrides: Partial<ReturnType<typeof useConfigStore.getState>>,
) {
  useConfigStore.setState({
    alwaysShowToc: false,
    setAlwaysShowToc,
    contentWidth: 'default',
    setContentWidth,
    ...overrides,
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  setAlwaysShowToc.mockResolvedValue(undefined)
  setContentWidth.mockResolvedValue(undefined)
  // jsdom has no real layout/pointer-capture engine, which Radix's Select
  // relies on when opening its popover — stub the bits it touches.
  window.HTMLElement.prototype.scrollIntoView = vi.fn()
  window.HTMLElement.prototype.hasPointerCapture = vi
    .fn()
    .mockReturnValue(false)
  window.HTMLElement.prototype.releasePointerCapture = vi.fn()
})

describe('DisplaySettings', () => {
  it.each([
    { state: 'off', alwaysShowToc: false },
    { state: 'on', alwaysShowToc: true },
  ])('reflects the current $state state', ({ alwaysShowToc }) => {
    setConfig({ alwaysShowToc })
    render(<DisplaySettings />)

    expect(
      screen.getByRole('checkbox', { checked: alwaysShowToc }),
    ).toHaveAttribute('data-testid', 'display-toc-checkbox')
  })

  it.each([
    { direction: 'on', from: false },
    { direction: 'off', from: true },
  ])(
    'toggling $direction applies immediately and shows a success toast',
    async ({ from }) => {
      const user = userEvent.setup({ pointerEventsCheck: 0 })
      setConfig({ alwaysShowToc: from })
      render(<DisplaySettings />)

      await user.click(screen.getByTestId('display-toc-checkbox'))
      expect(setAlwaysShowToc).toHaveBeenCalledWith(!from)
      expect(toast.success).toHaveBeenCalled()
    },
  )

  it('shows an error toast when the update fails', async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 })
    setAlwaysShowToc.mockRejectedValueOnce(new Error('boom'))
    setConfig({ alwaysShowToc: false })
    render(<DisplaySettings />)

    await user.click(screen.getByTestId('display-toc-checkbox'))
    expect(toast.error).toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
  })

  it('labels the page width picker and shows the current width', () => {
    setConfig({ contentWidth: 'wide' })
    render(<DisplaySettings />)

    expect(
      screen.getByRole('combobox', { name: 'Width of the page content' }),
    ).toHaveTextContent('Wide')
  })

  it('gives the width field a label distinct from its section title', () => {
    render(<DisplaySettings />)

    expect(screen.getAllByText('Page width')).toHaveLength(1)
    expect(
      screen.getByRole('heading', { name: 'Page width' }),
    ).toBeInTheDocument()
  })

  it('picking a width applies immediately and shows a success toast', async () => {
    const user = userEvent.setup({ delay: null, pointerEventsCheck: 0 })
    setConfig({ contentWidth: 'default' })
    render(<DisplaySettings />)

    await user.click(screen.getByTestId('display-content-width-select'))
    await user.click(await screen.findByRole('option', { name: 'Wide' }))

    expect(setContentWidth).toHaveBeenCalledWith('wide')
    expect(toast.success).toHaveBeenCalledWith('Page width saved')
  })

  it('shows an error toast and keeps the old width when saving the width fails', async () => {
    const user = userEvent.setup({ delay: null, pointerEventsCheck: 0 })
    setContentWidth.mockRejectedValueOnce(new Error('boom'))
    setConfig({ contentWidth: 'default' })
    render(<DisplaySettings />)

    await user.click(screen.getByTestId('display-content-width-select'))
    await user.click(await screen.findByRole('option', { name: 'Wide' }))

    expect(toast.error).toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
    expect(
      screen.getByTestId('display-content-width-select'),
    ).toHaveTextContent('Default')
  })
  it('picking the width that is already set sends no request', async () => {
    const user = userEvent.setup({ delay: null, pointerEventsCheck: 0 })
    setConfig({ contentWidth: 'wide' })
    render(<DisplaySettings />)

    await user.click(screen.getByTestId('display-content-width-select'))
    await user.click(await screen.findByRole('option', { name: 'Wide' }))

    expect(setContentWidth).not.toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
  })

  it('translates the server rejection of an unknown width', async () => {
    const user = userEvent.setup({ delay: null, pointerEventsCheck: 0 })
    setContentWidth.mockRejectedValueOnce(
      new ApiLocalizedError({
        code: 'page_display_invalid_content_width',
        message: 'Invalid page content width',
        template: 'invalid page content width',
        args: ['wide'],
      }),
    )
    setConfig({ contentWidth: 'default' })
    render(<DisplaySettings />)

    await user.click(screen.getByTestId('display-content-width-select'))
    await user.click(await screen.findByRole('option', { name: 'Wide' }))

    expect(toast.error).toHaveBeenCalledWith(
      expect.stringContaining('Invalid page width'),
    )
  })
})
