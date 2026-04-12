import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { API_BASE_URL } from '../config'
import { ApiLocalizedError, isApiLocalizedErrorResponse } from './errors'

export type AuthResponse = {
  message: string
  user: {
    id: string
    username: string
    email: string
    role: 'admin' | 'editor' | 'viewer'
  }
}

export class ApiError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

function getCsrfTokenFromCookie(): string | null {
  if (typeof document === 'undefined') return null

  const hostMatch =
    document.cookie.match(/(?:^|;\s*)__Host-leafwiki_csrf=([^;]+)/) ??
    document.cookie.match(/(?:^|;\s*)leafwiki_csrf=([^;]+)/)

  if (!hostMatch) return null
  try {
    return decodeURIComponent(hostMatch[1])
  } catch {
    return hostMatch[1]
  }
}

export async function login(identifier: string, password: string) {
  const res = await fetch(`${API_BASE_URL}/api/auth/login`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ identifier, password }),
  })

  if (!res.ok) {
    let errorBody: { error?: string } | null = null
    try {
      errorBody = await res.json()
    } catch {
      throw new Error('Login failed')
    }

    if (errorBody?.error) throw new Error(errorBody.error)
    throw new Error('Login failed')
  }

  const data: AuthResponse = await res.json()

  const { setUser } = useSessionStore.getState()
  setUser(data.user)

  return data
}

export async function logout() {
  const { authDisabled } = useConfigStore.getState()
  if (authDisabled) return
  const headers = new Headers()
  const csrfToken = getCsrfTokenFromCookie()
  if (csrfToken) headers.set('X-CSRF-Token', csrfToken)

  await fetch(`${API_BASE_URL}/api/auth/logout`, {
    method: 'POST',
    credentials: 'include',
    headers,
  }).catch(() => {})
}

export async function fetchWithAuth(
  path: string,
  options: RequestInit = {},
  retry = true,
): Promise<unknown> {
  const store = useSessionStore.getState()
  const sessionLogout = store.logout
  const authDisabled = useConfigStore.getState().authDisabled

  const headers = new Headers(options.headers || {})
  if (!(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  const method = (options.method || 'GET').toUpperCase()
  if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
    const csrfToken = getCsrfTokenFromCookie()
    if (csrfToken) {
      headers.set('X-CSRF-Token', csrfToken)
    }
  }

  let originalBody = options.body
  if (
    options.body &&
    typeof options.body === 'object' &&
    !(options.body instanceof FormData)
  ) {
    originalBody = JSON.stringify(options.body)
  }

  const doFetch = async (): Promise<Response> => {
    return fetch(`${API_BASE_URL}${path}`, {
      ...options,
      credentials: 'include',
      headers,
      body: originalBody,
    })
  }

  let res = await doFetch()

  if (res.status === 401 && retry && !authDisabled) {
    try {
      await ensureRefresh()
      res = await doFetch()
    } catch {
      if (!authDisabled) {
        sessionLogout()
        const { setUser } = useSessionStore.getState()
        setUser(null)
      }
      throw new Error('Unauthorized')
    }
  }

  if (!res.ok) {
    const errorText = await res.text()
    let errorBody: unknown = null
    try {
      errorBody = errorText ? JSON.parse(errorText) : null
    } catch {
      throw new ApiError(errorText || 'Request failed', res.status)
    }

    if (
      errorBody &&
      typeof errorBody === 'object' &&
      (errorBody as { error?: unknown }).error === 'validation_error'
    ) {
      throw errorBody
    }

    if (isApiLocalizedErrorResponse(errorBody)) {
      throw new ApiLocalizedError(errorBody.error)
    }

    if (
      errorBody &&
      typeof errorBody === 'object' &&
      typeof (errorBody as { error?: unknown }).error === 'string'
    ) {
      throw new ApiError((errorBody as { error: string }).error, res.status)
    }

    if (
      errorBody &&
      typeof errorBody === 'object' &&
      typeof (errorBody as { message?: unknown }).message === 'string'
    ) {
      throw new ApiError((errorBody as { message: string }).message, res.status)
    }

    throw new ApiError('Request failed', res.status)
  }

  try {
    return await res.json()
  } catch {
    return null
  }
}

declare global {
  var __leafwikiRefreshPromise: Promise<void> | null
}

if (typeof globalThis.__leafwikiRefreshPromise === 'undefined') {
  globalThis.__leafwikiRefreshPromise = null
}

export function ensureRefresh(): Promise<void> {
  const { authDisabled } = useConfigStore.getState()
  if (authDisabled) {
    return Promise.resolve()
  }

  if (!globalThis.__leafwikiRefreshPromise) {
    globalThis.__leafwikiRefreshPromise = refreshAccessToken().finally(() => {
      globalThis.__leafwikiRefreshPromise = null
    })
  }
  return globalThis.__leafwikiRefreshPromise
}

async function refreshAccessToken() {
  const store = useSessionStore.getState()

  const res = await fetch(`${API_BASE_URL}/api/auth/refresh-token`, {
    method: 'POST',
    credentials: 'include',
  })

  if (!res.ok) {
    throw new Error('Refresh failed')
  }

  const data = await res.json()
  store.setUser(data.user)
}
