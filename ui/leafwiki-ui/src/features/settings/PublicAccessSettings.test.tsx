import '@/lib/i18n'
import { useConfigStore } from '@/stores/config'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PublicAccessSettings from './PublicAccessSettings'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

import { toast } from 'sonner'

const setPublicAccess = vi.fn()

function setConfig(
  overrides: Partial<ReturnType<typeof useConfigStore.getState>>,
) {
  useConfigStore.setState({
    publicAccess: false,
    publicAccessEnvManaged: false,
    setPublicAccess,
    ...overrides,
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  setPublicAccess.mockResolvedValue(undefined)
})

describe('PublicAccessSettings', () => {
  it('env-managed: shows the status-only notice and no toggle control', () => {
    setConfig({ publicAccessEnvManaged: true, publicAccess: true })
    render(<PublicAccessSettings />)

    expect(screen.getByTestId('public-access-env-managed')).toBeInTheDocument()
    expect(screen.queryByTestId('public-access-enable')).not.toBeInTheDocument()
    expect(
      screen.queryByTestId('public-access-disable'),
    ).not.toBeInTheDocument()
  })

  it('settings-managed + off: enabling goes through a confirm dialog', async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 })
    setConfig({ publicAccess: false })
    render(<PublicAccessSettings />)

    await user.click(screen.getByTestId('public-access-enable'))
    // Not applied until the dialog is confirmed.
    expect(setPublicAccess).not.toHaveBeenCalled()

    await user.click(await screen.findByTestId('public-access-confirm'))
    expect(setPublicAccess).toHaveBeenCalledWith(true)
    expect(toast.success).toHaveBeenCalled()
  })

  it('settings-managed + on: disabling applies immediately without a dialog', async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 })
    setConfig({ publicAccess: true })
    render(<PublicAccessSettings />)

    await user.click(screen.getByTestId('public-access-disable'))
    expect(setPublicAccess).toHaveBeenCalledWith(false)
    expect(toast.success).toHaveBeenCalled()
  })

  it('shows an error toast when the update fails', async () => {
    const user = userEvent.setup({ pointerEventsCheck: 0 })
    setPublicAccess.mockRejectedValueOnce(new Error('boom'))
    setConfig({ publicAccess: true })
    render(<PublicAccessSettings />)

    await user.click(screen.getByTestId('public-access-disable'))
    expect(toast.error).toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
  })
})
