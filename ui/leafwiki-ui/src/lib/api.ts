import { useAuthStore } from '@/stores/auth'
import { API_BASE_URL } from './config'

export type AuthResponse = {
  token: string
  refresh_token: string
  user: {
    id: string
    username: string
    email: string
    role: 'admin' | 'editor'
  }
}

export function logout() {
  const { logout } = useAuthStore.getState()
  logout()
}

let isRefreshing = false
let refreshPromise: Promise<void> | null = null

export async function fetchWithAuth(
  path: string,
  options: RequestInit = {},
  retry = true,
): Promise<any> {
  const store = useAuthStore.getState()
  const token = store.token
  const logout = store.logout

  const headers = new Headers(options.headers || {})
  if (!(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers,
  })

  // Auto-Refresh bei 401
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
      return fetchWithAuth(path, options, false) // Retry once
    } catch {
      logout()
      throw new Error('Unauthorized')
    }
  }

  if (!res.ok) {
    let errorBody: any = null

    try {
      errorBody = await res.json()
    } catch {
      const text = await res.text()
      throw new Error(text || 'Request failed')
    }

    if (errorBody?.error === 'validation_error') {
      throw errorBody
    }

    if (errorBody?.error) {
      throw errorBody
    }

    throw new Error(errorBody?.message || 'Request failed')
  }

  try {
    return await res.json()
  } catch {
    return null
  }
}

async function refreshAccessToken() {
  const store = useAuthStore.getState()
  const refreshToken = store.refreshToken

  if (!refreshToken) throw new Error('No refresh token available')

  const res = await fetchWithAuth(`${API_BASE_URL}/api/auth/refresh-token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token: refreshToken }),
  })

  if (!res.ok) throw new Error('Refresh failed')

  const data = await res.json()
  store.setAuth(data.token, data.refresh_token, data.user)
}

export type PageNode = {
  id: string
  title: string
  slug: string
  path: string
  children: PageNode[]
}

export async function fetchTree(): Promise<PageNode> {
  try {
    return await fetchWithAuth(`/api/tree`)
  } catch (e) {
    throw new Error('Tree fetch failed')
  }
}

export async function suggestSlug(
  parentId: string,
  title: string,
): Promise<string> {
  try {
    const data = await fetchWithAuth(
      `/api/pages/slug-suggestion?parentID=${parentId}&title=${encodeURIComponent(title)}`,
    )
    return data.slug
  } catch (e) {
    throw new Error('Slug suggestion failed')
  }
}

export async function getPageByPath(path: string) {
  try {
    return await fetchWithAuth(
      `/api/pages/by-path?path=${encodeURIComponent(path)}`,
    )
  } catch (e) {
    throw new Error('Page not found')
  }
}

export async function createPage({
  title,
  slug,
  parentId,
}: {
  title: string
  slug: string
  parentId: string | null
}) {
  try {
    if (parentId === '') parentId = null
    return await fetchWithAuth(`/api/pages`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, slug, parentId }),
    })
  } catch (e) {
    throw e
  }
}

export async function updatePage(
  id: string,
  title: string,
  slug: string,
  content: string,
) {
  try {
    return await fetchWithAuth(`/api/pages/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, slug, content }),
    })
  } catch (e) {
    throw new Error('Page update failed')
  }
}

export async function deletePage(id: string) {
  try {
    return await fetchWithAuth(`/api/pages/${id}`, {
      method: 'DELETE',
    })
  } catch (e) {
    throw new Error('Page deletion failed')
  }
}

export async function movePage(id: string, parentId: string | null) {
  if (parentId === '' || parentId == 'root') parentId = null

  return await fetchWithAuth(`/api/pages/${id}/move`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ parentId }),
  })
}

export async function sortPages(parentId: string, orderedIDs: string[]) {
  try {
    if (parentId === '') parentId = 'root'

    return await fetchWithAuth(`/api/pages/${parentId}/sort`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ orderedIDs }),
    })
  } catch (e) {
    throw new Error('Sorting failed')
  }
}

export async function login(identifier: string, password: string) {
  const res = await fetch(`${API_BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ identifier, password }),
  })

  if (!res.ok) throw new Error('Login failed')

  const data: AuthResponse = await res.json()

  const { setAuth } = useAuthStore.getState()
  setAuth(data.token, data.refresh_token, data.user)

  return data
}

export type User = {
  id: string
  username: string
  email: string
  role: 'admin' | 'editor'
}

export async function getUsers(): Promise<User[]> {
  try {
    return await fetchWithAuth('/api/users')
  } catch (e) {
    throw new Error('User fetch failed')
  }
}

export async function createUser(
  user: Omit<User, 'id'> & { password: string },
) {
  try {
    await fetchWithAuth('/api/users', {
      method: 'POST',
      body: JSON.stringify(user),
    })
  } catch (e) {
    throw new Error('User creation failed')
  }
}

export async function updateUser(user: User & { password?: string }) {
  try {
    await fetchWithAuth(`/api/users/${user.id}`, {
      method: 'PUT',
      body: JSON.stringify(user),
    })
  } catch (e) {
    throw new Error('User update failed')
  }
}

export async function deleteUser(id: string) {
  try {
    return await fetchWithAuth(`/api/users/${id}`, {
      method: 'DELETE',
    })
  } catch (e) {
    throw new Error('User deletion failed')
  }
}

export async function uploadAsset(pageId: string, file: File) {
  const form = new FormData()
  form.append('file', file)
  try {
    return await fetchWithAuth(`/api/pages/${pageId}/assets`, {
      method: 'POST',
      body: form,
    })
  } catch (e) {
    throw new Error('Asset upload failed')
  }
}

export async function getAssets(pageId: string): Promise<string[]> {
  try {
    const data = await fetchWithAuth(`/api/pages/${pageId}/assets`, {})
    return data.files
  } catch (e) {
    throw new Error('Asset fetch failed')
  }
}

export async function deleteAsset(pageId: string, filename: string) {
  try {
    await fetchWithAuth(
      `/api/pages/${pageId}/assets/${encodeURIComponent(filename)}`,
      {
        method: 'DELETE',
      },
    )
  } catch (e) {
    throw new Error('Asset deletion failed')
  }
}
