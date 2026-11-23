import { DialogManager } from '@/components/DialogManager'
import { HotKeyHandler } from '@/components/HotKeyHandler'
import UserMenu from '@/components/UserMenu'
import * as authAPI from '@/lib/api/auth'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

describe('UserMenu', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  beforeEach(() => {
    vi.stubGlobal('__APP_VERSION__', 'test-version')

    useDialogsStore.setState({
      dialogType: null,
      dialogProps: null,
    })
    useConfigStore.setState({
      authDisabled: false,
      httpRemoteUserEnabled: false,
      loginUrl: '',
      logoutUrl: '',
    })
    useSessionStore.setState({
      user: {
        id: 'user-1',
        username: 'alice',
        email: 'alice@example.com',
        role: 'editor',
        totpEnabled: false,
      },
    })
  })

  it('navigates to /settings when the Settings item is clicked', async () => {
    const user = userEvent.setup({ delay: null })

    render(
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route path="/" element={<UserMenu />} />
          <Route
            path="/settings"
            element={<div data-testid="settings-page-sentinel">SETTINGS</div>}
          />
        </Routes>
      </MemoryRouter>,
    )

    const avatar = screen.getByTestId('user-menu-avatar')
    await user.click(avatar.closest('button') as HTMLButtonElement)
    await user.click(screen.getByTestId('user-menu-settings'))

    await waitFor(() => {
      expect(screen.getByTestId('settings-page-sentinel')).toBeInTheDocument()
    })
  })

  it('opens the shortcuts dialog from the user menu', async () => {
    const user = userEvent.setup({ delay: null })

    render(
      <MemoryRouter>
        <UserMenu />
        <DialogManager />
      </MemoryRouter>,
    )

    const avatar = screen.getByTestId('user-menu-avatar')
    const trigger = avatar.closest('button')

    expect(trigger).toBeTruthy()
    await user.click(trigger as HTMLButtonElement)

    const menuItem = await screen.findByText('Keyboard Shortcuts')
    await user.click(menuItem)

    await waitFor(() => {
      expect(screen.getByTestId('shortcuts-help-dialog')).toBeInTheDocument()
    })
    expect(screen.getByText('Available keyboard shortcuts')).toBeInTheDocument()
    expect(screen.getByText('Go to page')).toBeInTheDocument()
    expect(screen.getByText('Ctrl+/')).toBeInTheDocument()
  })

  it('opens the shortcuts dialog via the keyboard shortcut', async () => {
    render(
      <MemoryRouter>
        <UserMenu />
        <HotKeyHandler />
        <DialogManager />
      </MemoryRouter>,
    )

    fireEvent.keyDown(window, { key: '/', code: 'Slash', ctrlKey: true })

    await waitFor(() => {
      expect(screen.getByTestId('shortcuts-help-dialog')).toBeInTheDocument()
    })
  })

  it('opens the shortcuts dialog via keyboard shortcut in no-auth mode', async () => {
    useConfigStore.setState({ authDisabled: true })
    useSessionStore.setState({ user: null })

    render(
      <MemoryRouter>
        <UserMenu />
        <HotKeyHandler />
        <DialogManager />
      </MemoryRouter>,
    )

    fireEvent.keyDown(window, { key: '/', code: 'Slash', ctrlKey: true })

    await waitFor(() => {
      expect(screen.getByTestId('shortcuts-help-dialog')).toBeInTheDocument()
    })
  })

  describe('viewer role', () => {
    beforeEach(() => {
      useSessionStore.setState({
        user: {
          id: 'user-2',
          username: 'bob',
          email: 'bob@example.com',
          role: 'viewer',
          totpEnabled: false,
        },
      })
    })

    it('does not show the keyboard shortcuts menu item for viewer role', async () => {
      const user = userEvent.setup({ delay: null })

      render(
        <MemoryRouter>
          <UserMenu />
        </MemoryRouter>,
      )

      const avatar = screen.getByTestId('user-menu-avatar')
      const trigger = avatar.closest('button')
      await user.click(trigger as HTMLButtonElement)

      expect(screen.queryByText('Keyboard Shortcuts')).not.toBeInTheDocument()
    })

    it('still shows the Settings item for viewer role', async () => {
      const user = userEvent.setup({ delay: null })

      render(
        <MemoryRouter>
          <UserMenu />
        </MemoryRouter>,
      )

      const avatar = screen.getByTestId('user-menu-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)

      expect(screen.getByTestId('user-menu-settings')).toBeInTheDocument()
    })

    it('does not open the shortcuts dialog via keyboard shortcut for viewer role', async () => {
      render(
        <MemoryRouter>
          <UserMenu />
          <HotKeyHandler />
          <DialogManager />
        </MemoryRouter>,
      )

      fireEvent.keyDown(window, { key: '/', code: 'Slash', ctrlKey: true })

      await new Promise((r) => setTimeout(r, 100))
      expect(
        screen.queryByTestId('shortcuts-help-dialog'),
      ).not.toBeInTheDocument()
    })
  })

  describe('logout redirect', () => {
    const renderWithLoginRoute = () =>
      render(
        <MemoryRouter initialEntries={['/']}>
          <Routes>
            <Route path="/" element={<UserMenu />} />
            <Route
              path="/login"
              element={<div data-testid="login-form-sentinel">LOGIN</div>}
            />
          </Routes>
        </MemoryRouter>,
      )

    let originalLocation: Location

    beforeEach(() => {
      originalLocation = window.location
      Object.defineProperty(window, 'location', {
        configurable: true,
        value: { ...originalLocation, href: '' },
      })
    })

    afterEach(() => {
      Object.defineProperty(window, 'location', {
        configurable: true,
        value: originalLocation,
      })
    })

    it('redirects straight to the logout URL without rendering the local login screen', async () => {
      const user = userEvent.setup({ delay: null })
      const logoutSpy = vi.spyOn(authAPI, 'logout').mockResolvedValue()
      useConfigStore.setState({
        logoutUrl: 'https://control-plane.example.com/logout',
      })

      renderWithLoginRoute()

      const avatar = screen.getByTestId('user-menu-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)
      await user.click(screen.getByTestId('user-menu-logout'))

      expect(logoutSpy).toHaveBeenCalled()
      expect(window.location.href).toBe(
        'https://control-plane.example.com/logout',
      )
      expect(
        screen.queryByTestId('login-form-sentinel'),
      ).not.toBeInTheDocument()
    })

    it('falls back to the local /login route when no logout URL is configured', async () => {
      const user = userEvent.setup({ delay: null })
      useConfigStore.setState({ logoutUrl: '' })

      renderWithLoginRoute()

      const avatar = screen.getByTestId('user-menu-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)
      await user.click(screen.getByTestId('user-menu-logout'))

      await waitFor(() => {
        expect(screen.getByTestId('login-form-sentinel')).toBeInTheDocument()
      })
    })
  })

  describe('OIDC logout redirect', () => {
    const renderWithLoginRoute = () =>
      render(
        <MemoryRouter initialEntries={['/']}>
          <Routes>
            <Route path="/" element={<UserMenu />} />
            <Route
              path="/login"
              element={<div data-testid="login-form-sentinel">LOGIN</div>}
            />
          </Routes>
        </MemoryRouter>,
      )

    let originalLocation: Location

    beforeEach(() => {
      originalLocation = window.location
      Object.defineProperty(window, 'location', {
        configurable: true,
        value: { ...originalLocation, href: '' },
      })
      useSessionStore.setState({
        user: {
          id: 'user-1',
          username: 'alice',
          email: 'alice@example.com',
          role: 'editor',
          totpEnabled: false,
          authSource: 'oidc',
        },
      })
    })

    afterEach(() => {
      Object.defineProperty(window, 'location', {
        configurable: true,
        value: originalLocation,
      })
    })

    it('redirects to the IdP end-session URL, with client_id, post_logout_redirect_uri and id_token_hint, for an OIDC-linked user', async () => {
      const user = userEvent.setup({ delay: null })
      const logoutSpy = vi.spyOn(authAPI, 'logout').mockResolvedValue()
      const hintSpy = vi
        .spyOn(authAPI, 'getOidcLogoutHint')
        .mockResolvedValue('raw.id.token')
      useConfigStore.setState({
        oidcEndSessionUrl: 'https://idp.example.com/end-session',
        oidcClientId: 'leafwiki',
      })

      renderWithLoginRoute()

      const avatar = screen.getByTestId('user-menu-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)
      await user.click(screen.getByTestId('user-menu-logout'))

      expect(hintSpy).toHaveBeenCalled()
      expect(logoutSpy).toHaveBeenCalled()
      expect(window.location.href).toBe(
        'https://idp.example.com/end-session?client_id=leafwiki&post_logout_redirect_uri=' +
          encodeURIComponent(`${originalLocation.origin}/login`) +
          '&id_token_hint=raw.id.token',
      )
      expect(
        screen.queryByTestId('login-form-sentinel'),
      ).not.toBeInTheDocument()
    })

    // Regression test for a real production case: a session created before
    // id_token_hint existed has no hint to look up. Authentik rejects
    // post_logout_redirect_uri without id_token_hint outright once the
    // redirect URI is registered on the client — so without a hint, the
    // redirect URI must be omitted too, falling back to the IdP's own
    // "logged out" page (same as before id_token_hint existed at all).
    it('redirects without post_logout_redirect_uri or id_token_hint when the hint lookup comes back empty', async () => {
      const user = userEvent.setup({ delay: null })
      vi.spyOn(authAPI, 'logout').mockResolvedValue()
      vi.spyOn(authAPI, 'getOidcLogoutHint').mockResolvedValue('')
      useConfigStore.setState({
        oidcEndSessionUrl: 'https://idp.example.com/end-session',
        oidcClientId: 'leafwiki',
      })

      renderWithLoginRoute()

      const avatar = screen.getByTestId('user-menu-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)
      await user.click(screen.getByTestId('user-menu-logout'))

      expect(window.location.href).toBe(
        'https://idp.example.com/end-session?client_id=leafwiki',
      )
    })

    it('falls back to the local /login route when no end-session URL is configured', async () => {
      const user = userEvent.setup({ delay: null })
      useConfigStore.setState({ oidcEndSessionUrl: '' })

      renderWithLoginRoute()

      const avatar = screen.getByTestId('user-menu-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)
      await user.click(screen.getByTestId('user-menu-logout'))

      await waitFor(() => {
        expect(screen.getByTestId('login-form-sentinel')).toBeInTheDocument()
      })
    })

    it('prefers logoutUrl over the OIDC end-session URL when both are set', async () => {
      const user = userEvent.setup({ delay: null })
      vi.spyOn(authAPI, 'logout').mockResolvedValue()
      useConfigStore.setState({
        logoutUrl: 'https://control-plane.example.com/logout',
        oidcEndSessionUrl: 'https://idp.example.com/end-session',
      })

      renderWithLoginRoute()

      const avatar = screen.getByTestId('user-menu-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)
      await user.click(screen.getByTestId('user-menu-logout'))

      expect(window.location.href).toBe(
        'https://control-plane.example.com/logout',
      )
    })
  })

  describe('login redirect', () => {
    let originalLocation: Location

    beforeEach(() => {
      originalLocation = window.location
      Object.defineProperty(window, 'location', {
        configurable: true,
        value: { ...originalLocation, href: '' },
      })
      useSessionStore.setState({ user: null })
    })

    afterEach(() => {
      Object.defineProperty(window, 'location', {
        configurable: true,
        value: originalLocation,
      })
    })

    it('redirects to the login URL when clicking Login and loginUrl is configured', () => {
      useConfigStore.setState({
        loginUrl: 'https://idp.example.com/login',
      })

      render(
        <MemoryRouter>
          <UserMenu />
        </MemoryRouter>,
      )

      fireEvent.click(screen.getByRole('button', { name: 'Login' }))

      expect(window.location.href).toBe('https://idp.example.com/login')
    })
  })
})
