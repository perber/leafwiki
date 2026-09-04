import '@/lib/i18n'
import { DIALOG_USER_FORM } from '@/lib/registries'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { UserFormDialog } from './UserFormDialog'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn(), warning: vi.fn() },
}))

vi.mock('@/lib/api/users', () => ({
  getUsers: vi.fn().mockResolvedValue([]),
  createUser: vi.fn(),
  updateUser: vi.fn(),
  inviteUser: vi.fn(),
  resendInvite: vi.fn(),
  deleteUser: vi.fn(),
}))

import * as userAPI from '@/lib/api/users'
import { toast } from 'sonner'

describe('UserFormDialog invite mode', () => {
  beforeEach(() => {
    useDialogsStore.setState({
      dialogType: DIALOG_USER_FORM,
      dialogProps: null,
    })
    vi.clearAllMocks()
  })

  it('hides the mode toggle and password field is required when SMTP is disabled', () => {
    useConfigStore.setState({ smtpEnabled: false })
    render(<UserFormDialog />)

    expect(screen.queryByText('Send invite email')).not.toBeInTheDocument()
    expect(screen.getByPlaceholderText('password')).toBeInTheDocument()
  })

  it('shows the mode toggle when SMTP is enabled and hides the password field in invite mode', async () => {
    useConfigStore.setState({ smtpEnabled: true })
    const user = userEvent.setup()
    render(<UserFormDialog />)

    expect(screen.getByPlaceholderText('password')).toBeInTheDocument()

    await user.click(screen.getByText('Send invite email'))

    expect(screen.queryByPlaceholderText('password')).not.toBeInTheDocument()
  })

  it('calls inviteUser (not createUser) when submitting in invite mode', async () => {
    useConfigStore.setState({ smtpEnabled: true })
    ;(userAPI.inviteUser as ReturnType<typeof vi.fn>).mockResolvedValue({
      user: {
        id: 'u2',
        username: 'bob',
        email: 'bob@example.com',
        role: 'editor',
        totpEnabled: false,
        mustSetPassword: true,
      },
      emailSent: true,
    })

    const user = userEvent.setup()
    render(<UserFormDialog />)

    await user.click(screen.getByText('Send invite email'))
    await user.type(screen.getByPlaceholderText('username'), 'bob')
    await user.type(screen.getByPlaceholderText('email'), 'bob@example.com')
    await user.click(screen.getByTestId('user-form-dialog-button-confirm'))

    await waitFor(() =>
      expect(userAPI.inviteUser).toHaveBeenCalledWith({
        username: 'bob',
        email: 'bob@example.com',
        role: 'editor',
      }),
    )
    expect(userAPI.createUser).not.toHaveBeenCalled()
    expect(toast.success).toHaveBeenCalled()
  })

  it('warns instead of showing success when the invite email failed to send', async () => {
    useConfigStore.setState({ smtpEnabled: true })
    ;(userAPI.inviteUser as ReturnType<typeof vi.fn>).mockResolvedValue({
      user: {
        id: 'u2',
        username: 'bob',
        email: 'bob@example.com',
        role: 'editor',
        totpEnabled: false,
        mustSetPassword: true,
      },
      emailSent: false,
    })

    const user = userEvent.setup()
    render(<UserFormDialog />)

    await user.click(screen.getByText('Send invite email'))
    await user.type(screen.getByPlaceholderText('username'), 'bob')
    await user.type(screen.getByPlaceholderText('email'), 'bob@example.com')
    await user.click(screen.getByTestId('user-form-dialog-button-confirm'))

    await waitFor(() => expect(userAPI.inviteUser).toHaveBeenCalled())
    expect(toast.warning).toHaveBeenCalled()
    expect(toast.success).not.toHaveBeenCalled()
  })

  it('calls createUser (not inviteUser) when submitting in the default password mode', async () => {
    useConfigStore.setState({ smtpEnabled: true })
    ;(userAPI.createUser as ReturnType<typeof vi.fn>).mockResolvedValue({})

    const user = userEvent.setup()
    render(<UserFormDialog />)

    await user.type(screen.getByPlaceholderText('username'), 'carol')
    await user.type(screen.getByPlaceholderText('email'), 'carol@example.com')
    await user.type(screen.getByPlaceholderText('password'), 'a-password')
    await user.click(screen.getByTestId('user-form-dialog-button-confirm'))

    await waitFor(() => expect(userAPI.createUser).toHaveBeenCalled())
    expect(userAPI.inviteUser).not.toHaveBeenCalled()
  })
})
