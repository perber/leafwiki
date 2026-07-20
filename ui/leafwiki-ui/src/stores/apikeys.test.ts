import type { ApiKey, CreateApiKeyResult } from '@/lib/api/apikeys'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useApiKeyStore } from './apikeys'

vi.mock('@/lib/api/apikeys', () => ({
  getApiKeys: vi.fn(),
  createApiKey: vi.fn(),
  deleteApiKey: vi.fn(),
}))

import * as apiKeyAPI from '@/lib/api/apikeys'

const makeKey = (overrides: Partial<ApiKey> = {}): ApiKey => ({
  id: 'k1',
  name: 'agent key',
  userId: 'u1',
  prefix: 'ab12cd34',
  role: 'viewer',
  createdBy: 'admin1',
  createdAt: '2026-01-01T00:00:00Z',
  ...overrides,
})

describe('useApiKeyStore', () => {
  beforeEach(() => {
    useApiKeyStore.setState({ apiKeys: [] })
    vi.clearAllMocks()
  })

  it('loadApiKeys populates state from the API', async () => {
    const keys = [makeKey()]
    ;(apiKeyAPI.getApiKeys as ReturnType<typeof vi.fn>).mockResolvedValue(keys)

    await useApiKeyStore.getState().loadApiKeys()

    expect(useApiKeyStore.getState().apiKeys).toEqual(keys)
  })

  it('reset clears the list', () => {
    useApiKeyStore.setState({ apiKeys: [makeKey()] })
    useApiKeyStore.getState().reset()
    expect(useApiKeyStore.getState().apiKeys).toEqual([])
  })

  it('createApiKey returns the secret and refreshes the list', async () => {
    const created = makeKey({ id: 'k2' })
    const result: CreateApiKeyResult = {
      key: created,
      secret: 'lw_ab12cd34_secret',
    }
    ;(apiKeyAPI.createApiKey as ReturnType<typeof vi.fn>).mockResolvedValue(
      result,
    )
    ;(apiKeyAPI.getApiKeys as ReturnType<typeof vi.fn>).mockResolvedValue([
      created,
    ])

    const returned = await useApiKeyStore
      .getState()
      .createApiKey({ name: 'agent key', userId: 'u1' })

    expect(returned).toEqual(result)
    expect(apiKeyAPI.getApiKeys).toHaveBeenCalledTimes(1)
    expect(useApiKeyStore.getState().apiKeys).toEqual([created])
  })

  it('deleteApiKey removes the key and refreshes the list', async () => {
    useApiKeyStore.setState({ apiKeys: [makeKey()] })
    ;(apiKeyAPI.deleteApiKey as ReturnType<typeof vi.fn>).mockResolvedValue(
      undefined,
    )
    ;(apiKeyAPI.getApiKeys as ReturnType<typeof vi.fn>).mockResolvedValue([])

    await useApiKeyStore.getState().deleteApiKey('k1')

    expect(apiKeyAPI.deleteApiKey).toHaveBeenCalledWith('k1')
    expect(useApiKeyStore.getState().apiKeys).toEqual([])
  })
})
