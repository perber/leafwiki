import '@/lib/i18n'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LoginForm from './LoginForm'
import RequireAuth from './RequireAuth'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'

const loginMock = vi.fn()

vi.mock('@/lib/api/auth', () => ({
  login: (...args: unknown[]) => loginMock(...args),
}))

function LocationProbe() {
  const location = useLocation()
  const redirectTo =
    typeof location.state === 'object' &&
    location.state !== null &&
    'redirectTo' in location.state
      ? String((location.state as { redirectTo?: unknown }).redirectTo ?? '')
      : ''

  return (
    <>
      <div data-testid="pathname">{location.pathname}</div>
      <div data-testid="redirect-to">{redirectTo}</div>
    </>
  )
}

describe('Auth redirect flow', () => {
  beforeEach(() => {
    loginMock.mockReset()
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

  it('preserves the requested path when redirecting to login', async () => {
    render(
      <MemoryRouter
        initialEntries={[
          {
            pathname: '/home-lab/services-ip-addresses',
            search: '?tab=ports',
            hash: '#ssh',
          },
        ]}
      >
        <Routes>
          <Route path="/login" element={<LocationProbe />} />
          <Route
            path="*"
            element={
              <RequireAuth>
                <div>Protected page</div>
              </RequireAuth>
            }
          />
        </Routes>
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(screen.getByTestId('pathname')).toHaveTextContent('/login')
    })
    expect(screen.getByTestId('redirect-to')).toHaveTextContent(
      '/home-lab/services-ip-addresses?tab=ports#ssh',
    )
  })

  it('redirects externally instead of to /login when loginUrl is configured', async () => {
    const originalLocation = window.location
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...originalLocation, href: '' },
    })

    useConfigStore.setState({
      loginUrl: 'https://idp.example.com/login',
    })

    render(
      <MemoryRouter initialEntries={['/some/protected/page']}>
        <Routes>
          <Route path="/login" element={<LocationProbe />} />
          <Route
            path="*"
            element={
              <RequireAuth>
                <div>Protected page</div>
              </RequireAuth>
            }
          />
        </Routes>
      </MemoryRouter>,
    )

    await waitFor(() => {
      expect(window.location.href).toBe('https://idp.example.com/login')
    })
    expect(screen.queryByTestId('pathname')).not.toBeInTheDocument()

    Object.defineProperty(window, 'location', {
      configurable: true,
      value: originalLocation,
    })
  })

  it('returns to the requested page after successful login', async () => {
    loginMock.mockResolvedValue({
      accessTokenExpiresAt: 1234567890,
      message: 'ok',
      user: {
        id: 'user-1',
        username: 'admin',
        email: 'admin@example.com',
        role: 'admin',
      },
    })

    const user = userEvent.setup()

    render(
      <MemoryRouter
        initialEntries={[
          {
            pathname: '/login',
            state: {
              redirectTo: '/home-lab/services-ip-addresses?tab=ports#ssh',
            },
          },
        ]}
      >
        <Routes>
          <Route path="/login" element={<LoginForm />} />
          <Route path="/" element={<div>Home page</div>} />
          <Route
            path="/home-lab/services-ip-addresses"
            element={<div>Requested page</div>}
          />
        </Routes>
      </MemoryRouter>,
    )

    await user.type(screen.getByTestId('login-identifier'), 'admin')
    await user.type(screen.getByTestId('login-password'), 'admin')
    await user.click(screen.getByTestId('login-submit'))

    await screen.findByText('Requested page')
    expect(loginMock).toHaveBeenCalledWith('admin', 'admin')
  })
})
