import '@/lib/i18n'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
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
