import '@/lib/i18n'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LoginForm from './LoginForm'

const loginMock = vi.fn()
const completeTOTPLoginMock = vi.fn()

vi.mock('@/lib/api/auth', () => ({
  login: (...args: unknown[]) => loginMock(...args),
  completeTOTPLogin: (...args: unknown[]) => completeTOTPLoginMock(...args),
}))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

import { toast } from 'sonner'

const totpUser = {
  id: 'user-1',
  username: 'admin',
  email: 'admin@example.com',
  role: 'admin' as const,
  totpEnabled: true,
}

function renderLoginForm() {
  return render(
    <MemoryRouter initialEntries={['/login']}>
      <Routes>
        <Route path="/login" element={<LoginForm />} />
        <Route path="/" element={<div>Home page</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('LoginForm TOTP flow', () => {
  beforeEach(() => {
    loginMock.mockReset()
    completeTOTPLoginMock.mockReset()
    vi.mocked(toast.error).mockReset()
    useConfigStore.setState({
      authDisabled: false,
      httpRemoteUserEnabled: false,
    })
    useSessionStore.setState({
      user: null,
      isRefreshing: false,
      accessTokenExpiresAt: null,
    })
  })

  it('shows the TOTP code step instead of logging in when the account requires TOTP', async () => {
    loginMock.mockResolvedValue({
      requiresTotp: true,
      loginChallengeToken: 'challenge-token',
    })

    const user = userEvent.setup()
    renderLoginForm()

    await user.type(screen.getByTestId('login-identifier'), 'admin')
    await user.type(screen.getByTestId('login-password'), 'correct-password')
    await user.click(screen.getByTestId('login-submit'))

    expect(await screen.findByTestId('login-totp-code')).toBeInTheDocument()
    expect(screen.queryByTestId('login-identifier')).not.toBeInTheDocument()
  })

  it('completes login with a valid TOTP code and redirects', async () => {
    loginMock.mockResolvedValue({
      requiresTotp: true,
      loginChallengeToken: 'challenge-token',
    })
    completeTOTPLoginMock.mockResolvedValue({
      accessTokenExpiresAt: 1234567890,
      message: 'ok',
      user: totpUser,
    })

    const user = userEvent.setup()
    renderLoginForm()

    await user.type(screen.getByTestId('login-identifier'), 'admin')
    await user.type(screen.getByTestId('login-password'), 'correct-password')
    await user.click(screen.getByTestId('login-submit'))

    await user.type(await screen.findByTestId('login-totp-code'), '123456')
    await user.click(screen.getByTestId('login-totp-submit'))

    await screen.findByText('Home page')
    expect(completeTOTPLoginMock).toHaveBeenCalledWith(
      'challenge-token',
      '123456',
    )
  })

  it('accepts a recovery code in the same field as a TOTP code', async () => {
    loginMock.mockResolvedValue({
      requiresTotp: true,
      loginChallengeToken: 'challenge-token',
    })
    completeTOTPLoginMock.mockResolvedValue({
      accessTokenExpiresAt: 1234567890,
      message: 'ok',
      user: totpUser,
    })

    const user = userEvent.setup()
    renderLoginForm()

    await user.type(screen.getByTestId('login-identifier'), 'admin')
    await user.type(screen.getByTestId('login-password'), 'correct-password')
    await user.click(screen.getByTestId('login-submit'))

    await user.type(await screen.findByTestId('login-totp-code'), 'AB12-CD34')
    await user.click(screen.getByTestId('login-totp-submit'))

    await screen.findByText('Home page')
    expect(completeTOTPLoginMock).toHaveBeenCalledWith(
      'challenge-token',
      'AB12-CD34',
    )
  })

  it('shows an error and clears the code field when the TOTP code is rejected', async () => {
    loginMock.mockResolvedValue({
      requiresTotp: true,
      loginChallengeToken: 'challenge-token',
    })
    completeTOTPLoginMock.mockRejectedValue(new Error('invalid code'))

    const user = userEvent.setup()
    renderLoginForm()

    await user.type(screen.getByTestId('login-identifier'), 'admin')
    await user.type(screen.getByTestId('login-password'), 'correct-password')
    await user.click(screen.getByTestId('login-submit'))

    const codeInput = await screen.findByTestId('login-totp-code')
    await user.type(codeInput, '000000')
    await user.click(screen.getByTestId('login-totp-submit'))

    await waitFor(() => expect(toast.error).toHaveBeenCalled())
    expect(codeInput).toHaveValue('')
    // Still on the TOTP step, not redirected.
    expect(screen.queryByText('Home page')).not.toBeInTheDocument()
  })

  it('returns to the password step on "back", clearing the code but keeping the typed password', async () => {
    loginMock.mockResolvedValue({
      requiresTotp: true,
      loginChallengeToken: 'challenge-token',
    })

    const user = userEvent.setup()
    renderLoginForm()

    await user.type(screen.getByTestId('login-identifier'), 'admin')
    await user.type(screen.getByTestId('login-password'), 'correct-password')
    await user.click(screen.getByTestId('login-submit'))

    await screen.findByTestId('login-totp-code')
    await user.click(screen.getByRole('button', { name: 'Back' }))

    const passwordInput = await screen.findByTestId('login-password')
    // The password field is not cleared on "back" — resubmitting re-uses
    // the previously typed value without requiring it to be retyped.
    expect(passwordInput).toHaveValue('correct-password')
    expect(screen.queryByTestId('login-totp-code')).not.toBeInTheDocument()
  })

  it('preserves redirectTo across both login steps', async () => {
    loginMock.mockResolvedValue({
      requiresTotp: true,
      loginChallengeToken: 'challenge-token',
    })
    completeTOTPLoginMock.mockResolvedValue({
      accessTokenExpiresAt: 1234567890,
      message: 'ok',
      user: totpUser,
    })

    const user = userEvent.setup()
    render(
      <MemoryRouter
        initialEntries={[
          { pathname: '/login', state: { redirectTo: '/some/page' } },
        ]}
      >
        <Routes>
          <Route path="/login" element={<LoginForm />} />
          <Route path="/some/page" element={<div>Requested page</div>} />
        </Routes>
      </MemoryRouter>,
    )

    await user.type(screen.getByTestId('login-identifier'), 'admin')
    await user.type(screen.getByTestId('login-password'), 'correct-password')
    await user.click(screen.getByTestId('login-submit'))

    await user.type(await screen.findByTestId('login-totp-code'), '123456')
    await user.click(screen.getByTestId('login-totp-submit'))

    await screen.findByText('Requested page')
  })
})
