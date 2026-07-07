import { DIALOG_DELETE_API_KEY_CONFIRMATION } from '@/lib/registries'
import { useApiKeyStore } from '@/stores/apikeys'
import { useDialogsStore } from '@/stores/dialogs'
import { useUserStore } from '@/stores/users'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ApiKeysManagement from './ApiKeysManagement'

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
import * as userAPI from '@/lib/api/users'

describe('ApiKeysManagement', () => {
  beforeEach(() => {
    useApiKeyStore.setState({ apiKeys: [] })
    useUserStore.setState({ users: [] })
    useDialogsStore.setState({ dialogType: null, dialogProps: null })
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows a loading state then the empty state when there are no keys', async () => {
    ;(apiKeyAPI.getApiKeys as ReturnType<typeof vi.fn>).mockResolvedValue([])
    ;(userAPI.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue([])

    render(<ApiKeysManagement />)

    expect(screen.getByText('Loading API keys...')).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByText('No API keys found.')).toBeInTheDocument(),
    )
  })

  it('renders the list of keys with the owning username resolved', async () => {
    ;(apiKeyAPI.getApiKeys as ReturnType<typeof vi.fn>).mockResolvedValue([
      {
        id: 'k1',
        name: 'agent key',
        userId: 'u1',
        prefix: 'ab12cd34',
        role: 'viewer',
        createdBy: 'admin1',
        createdAt: '2026-01-01T00:00:00Z',
      },
    ])
    ;(userAPI.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 'u1', username: 'agent-owner', email: 'a@example.com', role: 'editor' },
    ])

    render(<ApiKeysManagement />)

    await waitFor(() =>
      expect(screen.getByText('agent key')).toBeInTheDocument(),
    )
    expect(screen.getByText('agent-owner')).toBeInTheDocument()
    expect(screen.getByText('viewer')).toBeInTheDocument()
  })

  it('shows "Revoked" instead of the revoke action for an already-revoked key', async () => {
    ;(apiKeyAPI.getApiKeys as ReturnType<typeof vi.fn>).mockResolvedValue([
      {
        id: 'k1',
        name: 'agent key',
        userId: 'u1',
        prefix: 'ab12cd34',
        role: 'viewer',
        createdBy: 'admin1',
        createdAt: '2026-01-01T00:00:00Z',
        revokedAt: '2026-01-02T00:00:00Z',
      },
    ])
    ;(userAPI.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue([])

    render(<ApiKeysManagement />)

    await waitFor(() => expect(screen.getByText('Revoked')).toBeInTheDocument())
    expect(screen.queryByText('Revoke')).not.toBeInTheDocument()
  })

  it('clicking Revoke opens the delete confirmation dialog for that key', async () => {
    const user = userEvent.setup()
    ;(apiKeyAPI.getApiKeys as ReturnType<typeof vi.fn>).mockResolvedValue([
      {
        id: 'k1',
        name: 'agent key',
        userId: 'u1',
        prefix: 'ab12cd34',
        role: 'viewer',
        createdBy: 'admin1',
        createdAt: '2026-01-01T00:00:00Z',
      },
    ])
    ;(userAPI.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue([])

    render(<ApiKeysManagement />)

    await waitFor(() =>
      expect(screen.getByText('agent key')).toBeInTheDocument(),
    )
    await user.click(screen.getByRole('button', { name: 'Revoke' }))

    expect(useDialogsStore.getState().dialogType).toBe(
      DIALOG_DELETE_API_KEY_CONFIRMATION,
    )
    expect(useDialogsStore.getState().dialogProps).toEqual({
      apiKeyId: 'k1',
      apiKeyName: 'agent key',
    })
  })
})
