import { DialogManager } from '@/components/DialogManager'
import UserToolbar from '@/components/UserToolbar'
import { useBackupStore } from '@/stores/backup'
import { useConfigStore } from '@/stores/config'
import { useDialogsStore } from '@/stores/dialogs'
import { useSessionStore } from '@/stores/session'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

describe('UserToolbar', () => {
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
  })
})
