import { DialogManager } from '@/components/DialogManager'
import { HotKeyHandler } from '@/components/HotKeyHandler'
import UserToolbar from '@/components/UserToolbar'
import { useBackupStore } from '@/stores/backup'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
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
      httpRemoteUserLogoutUrl: '',
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
})
