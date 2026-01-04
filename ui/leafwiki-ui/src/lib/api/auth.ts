import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { API_BASE_URL } from '../config'

export type AuthResponse = {
  message: string
  user: {
    id: string
    username: string
    email: string
    role: 'admin' | 'editor' | 'viewer'
  }
}

// Helper to get CSRF token from cookie
function getCsrfTokenFromCookie(): string | null {
  if (typeof document === 'undefined') return null

  // first try the __Host variant, then the "normal" one
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

  // Save the original body
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
      // Refresh token failed, log out the user
      if (!authDisabled) {
        sessionLogout()
        const { setUser } = useSessionStore.getState()
        setUser(null)
      }
      throw new Error('Unauthorized')
    }
  }

  if (!res.ok) {
    let errorBody: { error?: string; message?: string } | null = null
    try {
      errorBody = await res.json()
    } catch {
      const text = await res.text()
      throw new Error(text || 'Request failed')
    }

    if (errorBody?.error === 'validation_error') throw errorBody
    if (errorBody?.error) throw errorBody
    throw new Error(errorBody?.message || 'Request failed')
  }

  try {
    return await res.json()
  } catch {
    return null
  }
}

declare global {
  var __leafwikiRefreshPromise: Promise<void> | null | undefined
}

/**
 * Ensures there is only ONE refresh in-flight across the whole runtime (even if module is duplicated).
 */
export function ensureRefresh(): Promise<void> {
  const authDisabled = useConfigStore.getState().authDisabled
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
    // No logout here, handled in fetchWithAuth
    throw new Error('Refresh failed')
  }

  const data = await res.json()
  store.setUser(data.user)
}
