import { DialogManager } from '@/components/DialogManager'
import { HotKeyHandler } from '@/components/HotKeyHandler'
import UserToolbar from '@/components/UserToolbar'
import * as authAPI from '@/lib/api/auth'
import { useBackupStore } from '@/stores/backup'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

describe('UserToolbar', () => {
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
      userManagementUrl: '',
    })
    useBackupStore.setState({ enabled: false })
    useSessionStore.setState({
      user: {
        id: 'user-1',
        username: 'alice',
        email: 'alice@example.com',
        role: 'editor',
      },
    })
  })

  it('opens the shortcuts dialog from the user menu', async () => {
    const user = userEvent.setup()

    render(
      <MemoryRouter>
        <UserToolbar />
        <DialogManager />
      </MemoryRouter>,
    )

    const avatar = screen.getByTestId('user-toolbar-avatar')
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
        <UserToolbar />
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
        <UserToolbar />
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
        },
      })
    })

    it('does not show the keyboard shortcuts menu item for viewer role', async () => {
      const user = userEvent.setup()

      render(
        <MemoryRouter>
          <UserToolbar />
        </MemoryRouter>,
      )

      const avatar = screen.getByTestId('user-toolbar-avatar')
      const trigger = avatar.closest('button')
      await user.click(trigger as HTMLButtonElement)

      expect(screen.queryByText('Keyboard Shortcuts')).not.toBeInTheDocument()
    })

    it('does not open the shortcuts dialog via keyboard shortcut for viewer role', async () => {
      render(
        <MemoryRouter>
          <UserToolbar />
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
            <Route path="/" element={<UserToolbar />} />
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
      const user = userEvent.setup()
      const logoutSpy = vi.spyOn(authAPI, 'logout').mockResolvedValue()
      useConfigStore.setState({
        logoutUrl: 'https://control-plane.example.com/logout',
      })

      renderWithLoginRoute()

      const avatar = screen.getByTestId('user-toolbar-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)
      await user.click(screen.getByTestId('user-toolbar-logout'))

      expect(logoutSpy).toHaveBeenCalled()
      expect(window.location.href).toBe(
        'https://control-plane.example.com/logout',
      )
      expect(
        screen.queryByTestId('login-form-sentinel'),
      ).not.toBeInTheDocument()
    })

    it('falls back to the local /login route when no logout URL is configured', async () => {
      const user = userEvent.setup()
      useConfigStore.setState({ logoutUrl: '' })

      renderWithLoginRoute()

      const avatar = screen.getByTestId('user-toolbar-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)
      await user.click(screen.getByTestId('user-toolbar-logout'))

      await waitFor(() => {
        expect(screen.getByTestId('login-form-sentinel')).toBeInTheDocument()
      })
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
          <UserToolbar />
        </MemoryRouter>,
      )

      fireEvent.click(screen.getByRole('button', { name: 'Login' }))

      expect(window.location.href).toBe('https://idp.example.com/login')
    })
  })

  describe('user management link', () => {
    beforeEach(() => {
      useSessionStore.setState({
        user: {
          id: 'admin-1',
          username: 'admin',
          email: 'admin@example.com',
          role: 'admin',
        },
      })
    })

    it('renders User Management as an external link when userManagementUrl is set', async () => {
      const user = userEvent.setup()
      useConfigStore.setState({
        userManagementUrl: 'https://control-plane.example.com/users',
      })

      render(
        <MemoryRouter>
          <UserToolbar />
        </MemoryRouter>,
      )

      const avatar = screen.getByTestId('user-toolbar-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)

      const link = screen.getByText('User Management').closest('a')
      expect(link).toHaveAttribute(
        'href',
        'https://control-plane.example.com/users',
      )
      expect(link).toHaveAttribute('target', '_blank')
    })

    it('navigates to the local /users route when userManagementUrl is not set', async () => {
      const user = userEvent.setup()
      useConfigStore.setState({ userManagementUrl: '' })

      render(
        <MemoryRouter initialEntries={['/']}>
          <Routes>
            <Route path="/" element={<UserToolbar />} />
            <Route
              path="/users"
              element={<div data-testid="users-page-sentinel">USERS</div>}
            />
          </Routes>
        </MemoryRouter>,
      )

      const avatar = screen.getByTestId('user-toolbar-avatar')
      await user.click(avatar.closest('button') as HTMLButtonElement)
      await user.click(screen.getByText('User Management'))

      await waitFor(() => {
        expect(screen.getByTestId('users-page-sentinel')).toBeInTheDocument()
      })
    })
  })
})
