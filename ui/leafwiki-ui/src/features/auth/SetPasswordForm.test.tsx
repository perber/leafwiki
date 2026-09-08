import '@/lib/i18n'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SetPasswordForm } from './SetPasswordForm'

const confirmPasswordResetMock = vi.fn()
const acceptInviteMock = vi.fn()

vi.mock('@/lib/api/auth', () => ({
  confirmPasswordReset: (...args: unknown[]) =>
    confirmPasswordResetMock(...args),
  acceptInvite: (...args: unknown[]) => acceptInviteMock(...args),
}))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

// The backend rejects a too-short password with this body shape, which is
// NOT the localized-error envelope — see asApiValidationError.
const validationErrorBody = {
  error: 'validation_error',
  fields: [
    {
      field: 'newPassword',
      message: 'New password must be at least 8 characters long',
    },
  ],
}

function renderResetForm(path = '/reset-password?token=abc.def') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route
          path="/reset-password"
          element={<SetPasswordForm mode="reset" />}
        />
        <Route path="/login" element={<div>Login page</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

function renderInviteForm(path = '/accept-invite?token=abc.def') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route
          path="/accept-invite"
          element={<SetPasswordForm mode="invite" />}
        />
        <Route path="/" element={<div>Home page</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('SetPasswordForm reset mode', () => {
  beforeEach(() => {
    confirmPasswordResetMock.mockReset()
    acceptInviteMock.mockReset()
    vi.mocked(toast.error).mockReset()
  })

  it('redirects to /login when no token is present in the URL', () => {
    renderResetForm('/reset-password')
    expect(screen.getByText('Login page')).toBeInTheDocument()
  })

  it('rejects mismatched passwords without calling the API', async () => {
    const user = userEvent.setup()
    renderResetForm()

    await user.type(screen.getByTestId('set-password-new'), 'password-one')
    await user.type(screen.getByTestId('set-password-confirm'), 'password-two')
    await user.click(screen.getByTestId('set-password-submit'))

    expect(screen.getByText('Passwords do not match')).toBeInTheDocument()
    expect(confirmPasswordResetMock).not.toHaveBeenCalled()
  })

  it('rejects a too-short password inline without calling the API', async () => {
    const user = userEvent.setup()
    renderResetForm()

    await user.type(screen.getByTestId('set-password-new'), 'short')
    await user.type(screen.getByTestId('set-password-confirm'), 'short')
    await user.click(screen.getByTestId('set-password-submit'))

    expect(
      screen.getByText('Password must be at least 8 characters long'),
    ).toBeInTheDocument()
    expect(confirmPasswordResetMock).not.toHaveBeenCalled()
  })

  it('surfaces a readable message (not "validation_error") when the API rejects the password', async () => {
    confirmPasswordResetMock.mockRejectedValue(validationErrorBody)
    const user = userEvent.setup()
    renderResetForm()

    await user.type(screen.getByTestId('set-password-new'), 'a-new-password')
    await user.type(
      screen.getByTestId('set-password-confirm'),
      'a-new-password',
    )
    await user.click(screen.getByTestId('set-password-submit'))

    await waitFor(() => expect(confirmPasswordResetMock).toHaveBeenCalledOnce())
    // handleFieldErrors renders the backend's per-field message verbatim.
    expect(
      await screen.findByText(validationErrorBody.fields[0].message),
    ).toBeInTheDocument()
    expect(screen.queryByText('validation_error')).not.toBeInTheDocument()
    expect(toast.error).not.toHaveBeenCalledWith('validation_error')
  })

  it('shows a success message and does NOT navigate home on success', async () => {
    confirmPasswordResetMock.mockResolvedValue({
      user: {
        id: 'u1',
        username: 'alice',
        email: 'a@x.com',
        role: 'editor',
        totpEnabled: false,
      },
    })
    const user = userEvent.setup()
    renderResetForm()

    await user.type(screen.getByTestId('set-password-new'), 'a-new-password')
    await user.type(
      screen.getByTestId('set-password-confirm'),
      'a-new-password',
    )
    await user.click(screen.getByTestId('set-password-submit'))

    await waitFor(() =>
      expect(confirmPasswordResetMock).toHaveBeenCalledWith(
        'abc.def',
        'a-new-password',
      ),
    )
    expect(await screen.findByText('Password reset')).toBeInTheDocument()
  })
})

describe('SetPasswordForm invite mode', () => {
  beforeEach(() => {
    confirmPasswordResetMock.mockReset()
    acceptInviteMock.mockReset()
    vi.mocked(toast.error).mockReset()
  })

  it('surfaces a readable message (not "validation_error") when the API rejects the password', async () => {
    acceptInviteMock.mockRejectedValue(validationErrorBody)
    const user = userEvent.setup()
    renderInviteForm()

    await user.type(screen.getByTestId('set-password-new'), 'a-new-password')
    await user.type(
      screen.getByTestId('set-password-confirm'),
      'a-new-password',
    )
    await user.click(screen.getByTestId('set-password-submit'))

    await waitFor(() => expect(acceptInviteMock).toHaveBeenCalledOnce())
    expect(
      await screen.findByText(validationErrorBody.fields[0].message),
    ).toBeInTheDocument()
    expect(screen.queryByText('validation_error')).not.toBeInTheDocument()
  })

  it('calls acceptInvite and navigates home on success', async () => {
    acceptInviteMock.mockResolvedValue({
      accessTokenExpiresAt: 123,
      message: 'ok',
      user: {
        id: 'u1',
        username: 'bob',
        email: 'b@x.com',
        role: 'viewer',
        totpEnabled: false,
      },
    })
    const user = userEvent.setup()
    renderInviteForm()

    await user.type(screen.getByTestId('set-password-new'), 'a-new-password')
    await user.type(
      screen.getByTestId('set-password-confirm'),
      'a-new-password',
    )
    await user.click(screen.getByTestId('set-password-submit'))

    await waitFor(() =>
      expect(acceptInviteMock).toHaveBeenCalledWith(
        'abc.def',
        'a-new-password',
      ),
    )
    expect(await screen.findByText('Home page')).toBeInTheDocument()
  })
})
