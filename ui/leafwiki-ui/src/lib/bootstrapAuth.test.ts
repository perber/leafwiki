import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest'
import { ApiError, ensureRefresh, fetchMe } from './api/auth'
import { useBootstrapAuth } from './bootstrapAuth'

vi.mock('./api/auth', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./api/auth')>()
  return {
    ...actual,
    fetchMe: vi.fn(),
    ensureRefresh: vi.fn(),
  }
})

const testUser = {
  id: 'user-1',
  username: 'ivan',
  email: 'ivan@example.com',
  role: 'admin' as const,
  totpEnabled: false,
}

beforeEach(() => {
  vi.clearAllMocks()
  ;(fetchMe as Mock).mockResolvedValue(null)
  ;(ensureRefresh as Mock).mockResolvedValue(undefined)
  useConfigStore.setState({
    hasLoaded: true,
    configLoadSucceeded: true,
    httpRemoteUserEnabled: false,
    authDisabled: false,
  })
  useSessionStore.setState({ user: null, accessTokenExpiresAt: null })
})

describe('useBootstrapAuth', () => {
  it('calls fetchMe (not ensureRefresh) when config failed to load', async () => {
    useConfigStore.setState({
      configLoadSucceeded: false,
      httpRemoteUserEnabled: false,
    })
    useSessionStore.setState({ user: testUser })

    renderHook(() => useBootstrapAuth(true))

    await waitFor(() => expect(fetchMe).toHaveBeenCalled())
    expect(ensureRefresh).not.toHaveBeenCalled()
  })

  it('calls fetchMe when httpRemoteUserEnabled is true', async () => {
    useConfigStore.setState({
      configLoadSucceeded: true,
      httpRemoteUserEnabled: true,
    })

    renderHook(() => useBootstrapAuth(true))

    await waitFor(() => expect(fetchMe).toHaveBeenCalled())
    expect(ensureRefresh).not.toHaveBeenCalled()
  })

  it('calls ensureRefresh for a confirmed session-auth deployment', async () => {
    useConfigStore.setState({
      configLoadSucceeded: true,
      httpRemoteUserEnabled: false,
    })

    renderHook(() => useBootstrapAuth(true))

    await waitFor(() => expect(ensureRefresh).toHaveBeenCalled())
    expect(fetchMe).not.toHaveBeenCalled()
  })

  it('clears the user when fetchMe fails with an auth error', async () => {
    useConfigStore.setState({
      configLoadSucceeded: true,
      httpRemoteUserEnabled: true,
    })
    useSessionStore.setState({ user: testUser })
    ;(fetchMe as Mock).mockRejectedValue(new ApiError('unauthorized', 401))

    renderHook(() => useBootstrapAuth(true))

    await waitFor(() => expect(useSessionStore.getState().user).toBeNull())
  })

  it('preserves the user when fetchMe fails with a server/network error', async () => {
    useConfigStore.setState({
      configLoadSucceeded: true,
      httpRemoteUserEnabled: true,
    })
    useSessionStore.setState({ user: testUser })
    ;(fetchMe as Mock).mockRejectedValue(new Error('network error'))

    renderHook(() => useBootstrapAuth(true))

    await waitFor(() =>
      expect(useSessionStore.getState().isRefreshing).toBe(false),
    )
    expect(useSessionStore.getState().user).toEqual(testUser)
  })
})
