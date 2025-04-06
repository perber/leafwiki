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
  headers.set("Content-Type", "application/json")
  if (token) headers.set("Authorization", `Bearer ${token}`)

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
      throw new Error("Unauthorized")
    }
  }

  if (!res.ok) {
    const error = await res.text()
    throw new Error(error || "Request failed")
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

  if (!refreshToken) throw new Error("No refresh token available")

  const res = await fetchWithAuth(`${API_BASE_URL}/api/auth/refresh-token`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token: refreshToken }),
  })

  if (!res.ok) throw new Error("Refresh failed")

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
    throw new Error("Tree fetch failed")
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
    throw new Error("Slug suggestion failed")
  }
}

export async function getPageByPath(path: string) {
  try {
    return await fetchWithAuth(
      `/api/pages/by-path?path=${encodeURIComponent(path)}`,
    )
  } catch (e) {
    throw new Error("Page not found")
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
    throw new Error("Page creation failed")
  }

}

export async function updatePage(id: string, title: string, slug: string, content: string) {
  try {
    return await fetchWithAuth(`/api/pages/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title, slug, content }),
    })
  } catch (e) {
    throw new Error("Page update failed")
  }
}

export async function deletePage(id: string) {
  try {
    return await fetchWithAuth(`/api/pages/${id}`, {
      method: "DELETE",
    })
  } catch (e) {
    throw new Error("Page deletion failed")
  }
}

export async function movePage(id: string, parentId: string | null) {

  if (parentId === '' || parentId == "root") parentId = null

  const res = await fetchWithAuth(`/api/pages/${id}/move`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ parentId }),
  })
  if (!res.ok) throw new Error("Move failed")
}

export async function sortPages(parentId: string, orderedIDs: string[]) {
  try {
    if (parentId === '') parentId = "root"

    return await fetchWithAuth(`/api/pages/${parentId}/sort`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ orderedIDs }),
    })
  } catch (e) {
    throw new Error("Sorting failed")
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
