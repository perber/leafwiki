import '@/lib/i18n'
import { useConfigStore } from '@/stores/config'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TocDisplaySettings from './TocDisplaySettings'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

import { toast } from 'sonner'

const setAlwaysShowToc = vi.fn()

function setConfig(
  overrides: Partial<ReturnType<typeof useConfigStore.getState>>,
) {
  useConfigStore.setState({
    alwaysShowToc: false,
    setAlwaysShowToc,
    ...overrides,
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  setAlwaysShowToc.mockResolvedValue(undefined)
})

describe('TocDisplaySettings', () => {
  it('reflects the current off state', () => {
    setConfig({ alwaysShowToc: false })
    render(<TocDisplaySettings />)

    expect(screen.getByTestId('toc-display-checkbox')).not.toBeChecked()
  })

  it('reflects the current on state', () => {
    setConfig({ alwaysShowToc: true })
    render(<TocDisplaySettings />)

    expect(screen.getByTestId('toc-display-checkbox')).toBeChecked()
  })

  it('toggling on applies immediately and shows a success toast', async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 })
    setConfig({ alwaysShowToc: false })
    render(<TocDisplaySettings />)

    await user.click(screen.getByTestId('toc-display-checkbox'))
    expect(setAlwaysShowToc).toHaveBeenCalledWith(true)
    expect(toast.success).toHaveBeenCalled()
  })

  it('toggling off applies immediately and shows a success toast', async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 })
    setConfig({ alwaysShowToc: true })
    render(<TocDisplaySettings />)

    await user.click(screen.getByTestId('toc-display-checkbox'))
    expect(setAlwaysShowToc).toHaveBeenCalledWith(false)
    expect(toast.success).toHaveBeenCalled()
  })

  it('shows an error toast when the update fails', async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 })
    setAlwaysShowToc.mockRejectedValueOnce(new Error('boom'))
    setConfig({ alwaysShowToc: false })
    render(<TocDisplaySettings />)

    await user.click(screen.getByTestId('toc-display-checkbox'))
    expect(toast.error).toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
  })
})
