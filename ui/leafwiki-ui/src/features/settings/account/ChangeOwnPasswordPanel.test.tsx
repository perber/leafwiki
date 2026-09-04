import '@/lib/i18n'
import { useSessionStore } from '@/stores/session'
import { useUserStore } from '@/stores/users'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ChangeOwnPasswordPanel } from './ChangeOwnPasswordPanel'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

import { toast } from 'sonner'

const sessionUser = {
  id: 'user-1',
  username: 'admin',
  email: 'admin@example.com',
  role: 'admin' as const,
  totpEnabled: false,
}

describe('ChangeOwnPasswordPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useSessionStore.setState({
      user: sessionUser,
      isRefreshing: false,
      accessTokenExpiresAt: null,
    })
  })

  it('saves the new password and clears the form on success', async () => {
    const user = userEvent.setup({ delay: null })
    const changeOwnPassword = vi.fn().mockResolvedValue(undefined)
    useUserStore.setState({ changeOwnPassword })

    render(<ChangeOwnPasswordPanel />)

    await user.type(
      screen.getByTestId('change-own-password-old'),
      'old-password',
    )
    await user.type(
      screen.getByTestId('change-own-password-new'),
      'new-password',
    )
    await user.type(
      screen.getByTestId('change-own-password-confirm'),
      'new-password',
    )
    await user.click(screen.getByTestId('change-own-password-save'))

    await waitFor(() =>
      expect(changeOwnPassword).toHaveBeenCalledWith(
        'old-password',
        'new-password',
      ),
    )
    expect(toast.success).toHaveBeenCalled()
    expect(screen.getByTestId('change-own-password-old')).toHaveValue('')
  })

  it('shows a validation error when the new password is too short', async () => {
    const user = userEvent.setup({ delay: null })
    render(<ChangeOwnPasswordPanel />)

    await user.type(screen.getByTestId('change-own-password-new'), 'short')

    expect(
      screen.getByText('Password must be at least 8 characters long'),
    ).toBeInTheDocument()
    expect(screen.getByTestId('change-own-password-save')).toBeDisabled()
  })

  it('shows a validation error when the confirmation does not match', async () => {
    const user = userEvent.setup({ delay: null })
    render(<ChangeOwnPasswordPanel />)

    await user.type(
      screen.getByTestId('change-own-password-new'),
      'new-password',
    )
    await user.type(
      screen.getByTestId('change-own-password-confirm'),
      'mismatched',
    )

    expect(screen.getByText('Passwords do not match')).toBeInTheDocument()
    expect(screen.getByTestId('change-own-password-save')).toBeDisabled()
  })

  it('disables save until old password, new password and matching confirmation are all filled in', async () => {
    const user = userEvent.setup({ delay: null })
    render(<ChangeOwnPasswordPanel />)

    expect(screen.getByTestId('change-own-password-save')).toBeDisabled()

    await user.type(
      screen.getByTestId('change-own-password-old'),
      'old-password',
    )
    await user.type(
      screen.getByTestId('change-own-password-new'),
      'new-password',
    )
    await user.type(
      screen.getByTestId('change-own-password-confirm'),
      'new-password',
    )

    expect(screen.getByTestId('change-own-password-save')).not.toBeDisabled()
  })

  it('clears the passwords on a failed save', async () => {
    const user = userEvent.setup({ delay: null })
    const changeOwnPassword = vi
      .fn()
      .mockRejectedValue(new Error('wrong password'))
    useUserStore.setState({ changeOwnPassword })

    render(<ChangeOwnPasswordPanel />)

    await user.type(
      screen.getByTestId('change-own-password-old'),
      'wrong-password',
    )
    await user.type(
      screen.getByTestId('change-own-password-new'),
      'new-password',
    )
    await user.type(
      screen.getByTestId('change-own-password-confirm'),
      'new-password',
    )
    await user.click(screen.getByTestId('change-own-password-save'))

    await waitFor(() => expect(changeOwnPassword).toHaveBeenCalled())
    expect(screen.getByTestId('change-own-password-old')).toHaveValue('')
    expect(screen.getByTestId('change-own-password-new')).toHaveValue('')
  })
})
