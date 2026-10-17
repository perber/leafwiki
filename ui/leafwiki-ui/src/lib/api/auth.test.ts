import '@/lib/i18n'
import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  ApiError,
  confirmPasswordReset,
  ensureRefresh,
  fetchWithAuth,
  login,
} from './auth'
import { ApiLocalizedError } from './errors'

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

  it('still attempts refresh when config is not confirmed (may be a wasted 422)', async () => {
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

  it('does not call the refresh endpoint when httpRemoteUserEnabled is true', async () => {
    useConfigStore.setState({
      httpRemoteUserEnabled: true,
      configLoadSucceeded: true,
    })
    const fetchMock = createFetchMock({})
    vi.stubGlobal('fetch', fetchMock)

    await expect(ensureRefresh()).resolves.toBeUndefined()

    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('does not call the refresh endpoint when auth is disabled', async () => {
    useConfigStore.setState({ authDisabled: true })
    const fetchMock = createFetchMock({})
    vi.stubGlobal('fetch', fetchMock)

    await expect(ensureRefresh()).resolves.toBeUndefined()

    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('calls the refresh endpoint for a confirmed session-auth deployment', async () => {
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

  it('still succeeds when a preemptive refresh fails and config has not loaded', async () => {
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

  it('surfaces a genuine 401 as an error without forcing logout when config has not loaded', async () => {
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

  it('retries and succeeds after a 401 for a confirmed session-auth deployment', async () => {
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

  it('still forces a logout when a confirmed session token is genuinely expired', async () => {
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

describe('confirmPasswordReset', () => {
  it('re-throws a field-validation body untouched instead of the literal "validation_error"', async () => {
    const body = {
      error: 'validation_error',
      fields: [
        {
          field: 'newPassword',
          message: 'New password must be at least 8 characters long',
        },
      ],
    }
    const fetchMock = createFetchMock({
      '/api/auth/password/reset': { status: 400, body },
    })
    vi.stubGlobal('fetch', fetchMock)

    const err = await confirmPasswordReset('tok', 'short').then(
      () => {
        throw new Error('expected confirmPasswordReset to reject')
      },
      (e: unknown) => e,
    )

    // Regression: it used to arrive as new Error("validation_error"), which
    // handleFieldErrors cannot route to a field.
    expect(err).not.toBeInstanceOf(Error)
    expect(err).toEqual(body)
  })
})

// Pins how each response shape surfaces to callers, for the authenticated
// transport (fetchWithAuth) and the unauthenticated login-style requests.
describe('API error decoding', () => {
  const localized = {
    error: {
      code: 'page_not_found',
      message: 'Page not found',
      template: 'Page not found',
    },
  }
  const validation = {
    error: 'validation_error',
    fields: [{ field: 'title', message: 'Title is required' }],
  }

  function stubRawResponse(status: number, text: string) {
    const fetchMock = vi.fn(
      async () => new Response(status === 204 ? null : text, { status }),
    )
    vi.stubGlobal('fetch', fetchMock)
    return fetchMock
  }

  async function rejection(promise: Promise<unknown>): Promise<unknown> {
    return promise.then(
      () => {
        throw new Error('expected request to reject')
      },
      (e: unknown) => e,
    )
  }

  beforeEach(() => {
    useConfigStore.setState({
      authDisabled: true,
      httpRemoteUserEnabled: false,
      configLoadSucceeded: true,
      hasLoaded: true,
    })
    useSessionStore.setState({ user: null, accessTokenExpiresAt: null })
  })

  describe('fetchWithAuth', () => {
    it('re-throws a validation body untouched', async () => {
      stubRawResponse(400, JSON.stringify(validation))
      expect(await rejection(fetchWithAuth('/api/x'))).toEqual(validation)
    })

    it('throws ApiLocalizedError for a localized error body', async () => {
      stubRawResponse(404, JSON.stringify(localized))
      const err = await rejection(fetchWithAuth('/api/x'))
      expect(err).toBeInstanceOf(ApiLocalizedError)
      expect((err as ApiLocalizedError).code).toBe('page_not_found')
    })

    it('throws ApiError with the status for a string error field', async () => {
      stubRawResponse(409, JSON.stringify({ error: 'conflict' }))
      const err = await rejection(fetchWithAuth('/api/x'))
      expect(err).toBeInstanceOf(ApiError)
      expect((err as ApiError).message).toBe('conflict')
      expect((err as ApiError).status).toBe(409)
    })

    it('keeps an empty string error field as the message', async () => {
      stubRawResponse(400, JSON.stringify({ error: '' }))
      const err = await rejection(fetchWithAuth('/api/x'))
      expect(err).toBeInstanceOf(ApiError)
      expect((err as ApiError).message).toBe('')
    })

    it('falls back to a string message field', async () => {
      stubRawResponse(500, JSON.stringify({ message: 'boom' }))
      const err = await rejection(fetchWithAuth('/api/x'))
      expect(err).toBeInstanceOf(ApiError)
      expect((err as ApiError).message).toBe('boom')
      expect((err as ApiError).status).toBe(500)
    })

    it('uses the raw text for a non-JSON error body', async () => {
      stubRawResponse(502, 'Bad Gateway')
      const err = await rejection(fetchWithAuth('/api/x'))
      expect(err).toBeInstanceOf(ApiError)
      expect((err as ApiError).message).toBe('Bad Gateway')
      expect((err as ApiError).status).toBe(502)
    })

    it('uses the generic message for an empty or unrecognized error body', async () => {
      stubRawResponse(500, '')
      const empty = await rejection(fetchWithAuth('/api/x'))
      stubRawResponse(500, JSON.stringify({ unexpected: true }))
      const unknownShape = await rejection(fetchWithAuth('/api/x'))
      for (const err of [empty, unknownShape]) {
        expect(err).toBeInstanceOf(ApiError)
        expect((err as ApiError).message).not.toBe('')
        expect((err as ApiError).status).toBe(500)
      }
      expect((empty as ApiError).message).toBe(
        (unknownShape as ApiError).message,
      )
    })

    it('resolves null for a successful response without a JSON body', async () => {
      stubRawResponse(204, '')
      expect(await fetchWithAuth('/api/x', { method: 'DELETE' })).toBeNull()
    })

    it('sends JSON content type, credentials and the CSRF header on writes', async () => {
      document.cookie = 'leafwiki_csrf=csrf-token-value'
      const fetchMock = stubRawResponse(200, '{}')
      await fetchWithAuth('/api/x', { method: 'POST', body: '{"a":1}' })
      const [url, init] = fetchMock.mock.calls[0] as unknown as [
        string,
        RequestInit,
      ]
      const headers = new Headers(init.headers)
      expect(url.endsWith('/api/x')).toBe(true)
      expect(init.credentials).toBe('include')
      expect(headers.get('Content-Type')).toBe('application/json')
      expect(headers.get('X-CSRF-Token')).toBe('csrf-token-value')
      expect(init.body).toBe('{"a":1}')
      document.cookie = 'leafwiki_csrf=; expires=Thu, 01 Jan 1970 00:00:00 GMT'
    })

    it('leaves the content type to the browser for FormData bodies', async () => {
      const fetchMock = stubRawResponse(200, '{}')
      const form = new FormData()
      await fetchWithAuth('/api/x', { method: 'POST', body: form })
      const [, init] = fetchMock.mock.calls[0] as unknown as [
        string,
        RequestInit,
      ]
      expect(new Headers(init.headers).has('Content-Type')).toBe(false)
      expect(init.body).toBe(form)
    })
  })

  describe('login requests', () => {
    it('re-throws a validation body untouched', async () => {
      stubRawResponse(400, JSON.stringify(validation))
      expect(await rejection(login('a', 'b'))).toEqual(validation)
    })

    it('throws ApiLocalizedError for a localized error body', async () => {
      stubRawResponse(401, JSON.stringify(localized))
      expect(await rejection(login('a', 'b'))).toBeInstanceOf(ApiLocalizedError)
    })

    it('throws a plain Error (not ApiError) for a string error field', async () => {
      stubRawResponse(401, JSON.stringify({ error: 'invalid credentials' }))
      const err = await rejection(login('a', 'b'))
      expect(err).toBeInstanceOf(Error)
      expect(err).not.toBeInstanceOf(ApiError)
      expect((err as Error).message).toBe('invalid credentials')
    })

    it('uses the login fallback for non-JSON and unrecognized bodies', async () => {
      stubRawResponse(500, 'oops')
      const nonJSON = await rejection(login('a', 'b'))
      stubRawResponse(500, JSON.stringify({ message: 'ignored' }))
      const unknownShape = await rejection(login('a', 'b'))
      for (const err of [nonJSON, unknownShape]) {
        expect(err).toBeInstanceOf(Error)
        expect(err).not.toBeInstanceOf(ApiError)
      }
      expect((nonJSON as Error).message).toBe((unknownShape as Error).message)
      expect((nonJSON as Error).message).not.toBe('ignored')
    })
  })
})
