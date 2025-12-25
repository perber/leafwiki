import { useSessionStore } from '@/stores/session'
import { API_BASE_URL } from '../config'

export type AuthResponse = {
  message: string
  user: {
    id: string
    username: string
    email: string
    role: 'admin' | 'editor'
  }
}

export async function login(identifier: string, password: string) {
  const res = await fetch(`${API_BASE_URL}/api/auth/login`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ identifier, password }),
  })

  if (!res.ok) throw new Error('Login failed')

  const data: AuthResponse = await res.json()

  const { setUser } = useSessionStore.getState()
  setUser(data.user)

  return data
}

export async function logout() {
  await fetch(`${API_BASE_URL}/api/auth/logout`, {
    method: 'POST',
    credentials: 'include',
  }).catch(() => {})

  const { logout } = useSessionStore.getState()
  logout()
}

let isRefreshing = false
let refreshPromise: Promise<void> | null = null

export async function fetchWithAuth(
  path: string,
  options: RequestInit = {},
  retry = true,
): Promise<unknown> {
  const store = useSessionStore.getState()
  const logout = store.logout

  const headers = new Headers(options.headers || {})
  if (!(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
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

  if (res.status === 401 && retry) {
    if (!isRefreshing) {
      isRefreshing = true
      refreshPromise = refreshAccessToken().finally(() => {
        isRefreshing = false
        refreshPromise = null
      })
    }

    try {
      await refreshPromise
      res = await doFetch()
    } catch {
      logout()
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

async function refreshAccessToken() {
  const store = useSessionStore.getState()
  const setRefreshing = useSessionStore.getState().setRefreshing

  setRefreshing(true)
  try {
    const res = await fetch(`${API_BASE_URL}/api/auth/refresh-token`, {
      method: 'POST',
      credentials: 'include',
    })

    if (!res.ok) throw new Error('Refresh failed')

    const data = await res.json()
    store.setUser(data.user)
  } finally {
    setRefreshing(false)
  }
}
