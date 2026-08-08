import '@/lib/i18n'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ensureRefresh, fetchWithAuth } from './auth'

type MockResponseSpec = {
  status: number
  body?: unknown
}

// Maps a request path suffix (e.g. '/api/auth/refresh-token') to one response,
// or a queue of responses consumed in order across repeated calls to the same
// path (used to simulate "401 then a successful retry").
function createFetchMock(
  routes: Record<string, MockResponseSpec | MockResponseSpec[]>,
) {
  const queues = new Map<string, MockResponseSpec[]>(
    Object.entries(routes).map(([path, spec]) => [
      path,
      Array.isArray(spec) ? [...spec] : [spec],
    ]),
  )

  return vi.fn(async (input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input.toString()
    const path = Object.keys(routes).find((p) => url.endsWith(p))
    if (!path) {
      return new Response(JSON.stringify({ error: 'unmapped_route' }), {
        status: 404,
      })
    }
    const queue = queues.get(path)!
    const spec = queue.length > 1 ? queue.shift()! : queue[0]
    return new Response(JSON.stringify(spec.body ?? {}), {
      status: spec.status,
      headers: { 'Content-Type': 'application/json' },
    })
  })
}

function calledPaths(fetchMock: ReturnType<typeof createFetchMock>): string[] {
  return fetchMock.mock.calls.map(([input]) =>
    typeof input === 'string' ? input : String(input),
  )
}

const testUser = {
  id: 'user-1',
  username: 'ivan',
  email: 'ivan@example.com',
  role: 'admin' as const,
  totpEnabled: false,
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('ensureRefresh', () => {
  beforeEach(() => {
    useConfigStore.setState({
      authDisabled: false,
      httpRemoteUserEnabled: false,
      configLoadSucceeded: true,
    })
    useSessionStore.setState({ user: null, accessTokenExpiresAt: null })
  })

  it('TestEnsureRefresh_ConfigLoadNotSucceeded_StillAttemptsRefresh', async () => {
    // Deliberate: attempting the call is harmless by itself (worst case one
    // wasted 422). Skipping it here entirely would permanently disable
    // refresh for session-auth deployments that hit one bad /api/config
    // fetch — see the comment on ensureRefresh() for the full reasoning.
    useConfigStore.setState({ configLoadSucceeded: false })
    const fetchMock = createFetchMock({
      '/api/auth/refresh-token': { status: 422 },
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(ensureRefresh()).rejects.toThrow()

    expect(
      calledPaths(fetchMock).some((p) => p.endsWith('/api/auth/refresh-token')),
    ).toBe(true)
  })

  it('TestEnsureRefresh_HttpRemoteUserEnabled_DoesNotCallRefreshEndpoint', async () => {
    useConfigStore.setState({
      httpRemoteUserEnabled: true,
      configLoadSucceeded: true,
    })
    const fetchMock = createFetchMock({})
    vi.stubGlobal('fetch', fetchMock)

    await expect(ensureRefresh()).resolves.toBeUndefined()

    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('TestEnsureRefresh_AuthDisabled_DoesNotCallRefreshEndpoint', async () => {
    useConfigStore.setState({ authDisabled: true })
    const fetchMock = createFetchMock({})
    vi.stubGlobal('fetch', fetchMock)

    await expect(ensureRefresh()).resolves.toBeUndefined()

    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('TestEnsureRefresh_SessionAuthConfirmed_CallsRefreshEndpoint', async () => {
    const fetchMock = createFetchMock({
      '/api/auth/refresh-token': {
        status: 200,
        body: {
          accessTokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
          message: 'ok',
          user: testUser,
        },
      },
    })
    vi.stubGlobal('fetch', fetchMock)

    await ensureRefresh()

    expect(
      calledPaths(fetchMock).some((p) => p.endsWith('/api/auth/refresh-token')),
    ).toBe(true)
  })
})

describe('fetchWithAuth', () => {
  beforeEach(() => {
    useConfigStore.setState({
      authDisabled: false,
      httpRemoteUserEnabled: false,
      configLoadSucceeded: true,
      hasLoaded: true,
    })
    useSessionStore.setState({ user: null, accessTokenExpiresAt: null })
  })

  it('TestFetchWithAuth_ConfigLoadFailed_PreemptiveRefreshFails_RequestStillSucceeds', async () => {
    // Reproduces the exact reported scenario (GitHub #1407): config failed to
    // load, a user is persisted from a prior header-auth session (so
    // accessTokenExpiresAt is null and shouldRefreshBeforeRequest fires on
    // every request), the speculative refresh 422s like it always would for
    // header-auth — but the real request must still go through and succeed,
    // and must not force a logout / touch the CSRF cookie.
    useConfigStore.setState({ configLoadSucceeded: false })
    useSessionStore.setState({ user: testUser, accessTokenExpiresAt: null })

    const fetchMock = createFetchMock({
      '/api/some/path': { status: 200, body: { ok: true } },
      '/api/auth/refresh-token': { status: 422 },
      '/api/auth/logout': { status: 200 },
    })
    vi.stubGlobal('fetch', fetchMock)

    const result = await fetchWithAuth('/api/some/path')

    expect(result).toEqual({ ok: true })
    const paths = calledPaths(fetchMock)
    expect(paths.some((p) => p.endsWith('/api/auth/refresh-token'))).toBe(true)
    expect(paths.some((p) => p.endsWith('/api/auth/logout'))).toBe(false)
    expect(useSessionStore.getState().user).toEqual(testUser)
  })

  it('TestFetchWithAuth_ConfigLoadFailed_Genuine401_SurfacesErrorWithoutLogout', async () => {
    useConfigStore.setState({ configLoadSucceeded: false })
    useSessionStore.setState({
      user: testUser,
      // Far enough in the future that shouldRefreshBeforeRequest skips the
      // preemptive path, isolating this test to the 401-retry branch.
      accessTokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
    })

    const fetchMock = createFetchMock({
      '/api/some/path': { status: 401 },
      '/api/auth/refresh-token': { status: 422 },
      '/api/auth/logout': { status: 200 },
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(fetchWithAuth('/api/some/path')).rejects.toThrow()

    expect(
      calledPaths(fetchMock).some((p) => p.endsWith('/api/auth/logout')),
    ).toBe(false)
    expect(useSessionStore.getState().user).toEqual(testUser)
  })

  it('TestFetchWithAuth_SessionAuthConfirmed_401RetryStillWorks', async () => {
    useConfigStore.setState({ configLoadSucceeded: true })
    useSessionStore.setState({
      user: testUser,
      accessTokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
    })

    const fetchMock = createFetchMock({
      '/api/some/path': [{ status: 401 }, { status: 200, body: { ok: true } }],
      '/api/auth/refresh-token': {
        status: 200,
        body: {
          accessTokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
          message: 'ok',
          user: testUser,
        },
      },
    })
    vi.stubGlobal('fetch', fetchMock)

    const result = await fetchWithAuth('/api/some/path')

    expect(result).toEqual({ ok: true })
    expect(
      calledPaths(fetchMock).filter((p) =>
        p.endsWith('/api/auth/refresh-token'),
      ),
    ).toHaveLength(1)
  })

  it('TestFetchWithAuth_SessionAuthConfirmed_ExpiredTokenStillForcesLogout', async () => {
    useConfigStore.setState({ configLoadSucceeded: true })
    useSessionStore.setState({
      user: testUser,
      accessTokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
    })

    const fetchMock = createFetchMock({
      '/api/some/path': { status: 401 },
      '/api/auth/refresh-token': { status: 422 },
      '/api/auth/logout': { status: 200 },
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(fetchWithAuth('/api/some/path')).rejects.toThrow()

    expect(
      calledPaths(fetchMock).some((p) => p.endsWith('/api/auth/logout')),
    ).toBe(true)
    expect(useSessionStore.getState().user).toBeNull()
  })
})
