import { DIALOG_API_KEY_FORM } from '@/lib/registries'
import { useApiKeyStore } from '@/stores/apikeys'
import { useDialogsStore } from '@/stores/dialogs'
import { useUserStore } from '@/stores/users'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiKeyFormDialog } from './ApiKeyFormDialog'

// Radix Select relies on jsdom APIs that jsdom doesn't implement.
beforeAll(() => {
  Element.prototype.hasPointerCapture = () => false
  Element.prototype.releasePointerCapture = () => {}
  Element.prototype.scrollIntoView = () => {}
})

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

vi.mock('@/lib/api/apikeys', () => ({
  getApiKeys: vi.fn(),
  createApiKey: vi.fn(),
  deleteApiKey: vi.fn(),
}))

vi.mock('@/lib/api/users', () => ({
  getUsers: vi.fn(),
  createUser: vi.fn(),
  updateUser: vi.fn(),
  deleteUser: vi.fn(),
}))

import * as apiKeyAPI from '@/lib/api/apikeys'

describe('ApiKeyFormDialog', () => {
  beforeEach(() => {
    useDialogsStore.setState({ dialogType: DIALOG_API_KEY_FORM, dialogProps: null })
    useApiKeyStore.setState({ apiKeys: [] })
    useUserStore.setState({
      users: [
        { id: 'u1', username: 'agent-owner', email: 'a@example.com', role: 'editor' },
      ],
    })
    vi.clearAllMocks()
  })

  it('disables Create until a name and an owner are set', async () => {
    const user = userEvent.setup()
    render(<ApiKeyFormDialog />)

    expect(screen.getByTestId('api-key-form-dialog-button-confirm')).toBeDisabled()

    await user.type(screen.getByPlaceholderText('e.g. research-agent'), 'agent key')
    expect(screen.getByTestId('api-key-form-dialog-button-confirm')).toBeDisabled()

    await user.click(screen.getByTestId('api-key-owner-select'))
    await user.click(await screen.findByRole('option', { name: 'agent-owner' }))

    expect(screen.getByTestId('api-key-form-dialog-button-confirm')).not.toBeDisabled()
  })

  it('reveals the one-time secret after creation and copies it', async () => {
    const user = userEvent.setup()
    // userEvent.setup() installs its own navigator.clipboard stub, so ours
    // must be defined after that call, not before.
    const clipboardWriteText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: clipboardWriteText },
      configurable: true,
    })
    ;(apiKeyAPI.createApiKey as ReturnType<typeof vi.fn>).mockResolvedValue({
      key: {
        id: 'k1',
        name: 'agent key',
        userId: 'u1',
        prefix: 'ab12cd34',
        role: 'viewer',
        createdBy: 'admin1',
        createdAt: '2026-01-01T00:00:00Z',
      },
      secret: 'lw_ab12cd34_supersecret',
    })
    ;(apiKeyAPI.getApiKeys as ReturnType<typeof vi.fn>).mockResolvedValue([])

    render(<ApiKeyFormDialog />)

    await user.type(screen.getByPlaceholderText('e.g. research-agent'), 'agent key')
    await user.click(screen.getByTestId('api-key-owner-select'))
    await user.click(await screen.findByRole('option', { name: 'agent-owner' }))
    await user.click(screen.getByTestId('api-key-form-dialog-button-confirm'))

    await waitFor(() =>
      expect(screen.getByTestId('api-key-secret')).toHaveValue(
        'lw_ab12cd34_supersecret',
      ),
    )
    expect(apiKeyAPI.createApiKey).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'agent key', userId: 'u1', role: 'viewer' }),
    )

    await user.click(screen.getByTestId('api-key-secret-copy'))
    expect(clipboardWriteText).toHaveBeenCalledWith('lw_ab12cd34_supersecret')

    // Confirm button is gone in the reveal state; only "Done" remains.
    expect(
      screen.queryByTestId('api-key-form-dialog-button-confirm'),
    ).not.toBeInTheDocument()
  })

  it('surfaces field validation errors from the API without closing the dialog', async () => {
    const user = userEvent.setup()
    ;(apiKeyAPI.createApiKey as ReturnType<typeof vi.fn>).mockRejectedValue({
      error: 'validation_error',
      fields: [{ field: 'name', message: 'Name must not be empty' }],
    })

    render(<ApiKeyFormDialog />)

    await user.type(screen.getByPlaceholderText('e.g. research-agent'), 'x')
    await user.click(screen.getByTestId('api-key-owner-select'))
    await user.click(await screen.findByRole('option', { name: 'agent-owner' }))
    await user.click(screen.getByTestId('api-key-form-dialog-button-confirm'))

    await waitFor(() =>
      expect(screen.getByText('Name must not be empty')).toBeInTheDocument(),
    )
    // Dialog stays open (secret never revealed).
    expect(screen.queryByTestId('api-key-secret')).not.toBeInTheDocument()
  })
})
