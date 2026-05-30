import { fetchWithAuth } from './auth'

export type User = {
  id: string
  username: string
  email: string
  role: 'admin' | 'editor' | 'viewer'
}

export type MCPAPIKey = {
  id: string
  userId: string
  name: string
  prefix: string
  last4: string
  scopes: string[]
  createdByUserId: string
  createdAt: string
  lastUsedAt: string | null
  revokedAt: string | null
}

export type MCPAPIKeyCreateResponse = {
  key: MCPAPIKey
  secret: string
}

export async function getUsers(): Promise<User[]> {
  return (await fetchWithAuth('/api/users')) as User[]
}

export async function createUser(
  user: Omit<User, 'id'> & { password: string },
) {
  return await fetchWithAuth('/api/users', {
    method: 'POST',
    body: JSON.stringify(user),
  })
}

export async function updateUser(user: User & { password?: string }) {
  return await fetchWithAuth(`/api/users/${user.id}`, {
    method: 'PUT',
    body: JSON.stringify(user),
  })
}

export async function changeOwnPassword(
  oldPassword: string,
  newPassword: string,
) {
  return await fetchWithAuth(`/api/users/me/password`, {
    method: 'PUT',
    body: JSON.stringify({
      oldPassword,
      newPassword,
    }),
  })
}

export async function deleteUser(id: string) {
  return await fetchWithAuth(`/api/users/${id}`, {
    method: 'DELETE',
  })
}

export async function getUserMCPAPIKeys(userId: string): Promise<MCPAPIKey[]> {
  return (await fetchWithAuth(
    `/api/users/${userId}/mcp-api-keys`,
  )) as MCPAPIKey[]
}

export async function createUserMCPAPIKey(
  userId: string,
  name: string,
): Promise<MCPAPIKeyCreateResponse> {
  return (await fetchWithAuth(`/api/users/${userId}/mcp-api-keys`, {
    method: 'POST',
    body: JSON.stringify({ name }),
  })) as MCPAPIKeyCreateResponse
}

export async function revokeUserMCPAPIKey(userId: string, keyId: string) {
  return await fetchWithAuth(`/api/users/${userId}/mcp-api-keys/${keyId}`, {
    method: 'DELETE',
  })
}

export async function getOwnMCPAPIKeys(): Promise<MCPAPIKey[]> {
  return (await fetchWithAuth('/api/users/me/mcp-api-keys')) as MCPAPIKey[]
}

export async function createOwnMCPAPIKey(
  name: string,
  currentPassword: string,
): Promise<MCPAPIKeyCreateResponse> {
  return (await fetchWithAuth('/api/users/me/mcp-api-keys', {
    method: 'POST',
    body: JSON.stringify({ name, currentPassword }),
  })) as MCPAPIKeyCreateResponse
}

export async function revokeOwnMCPAPIKey(keyId: string) {
  return await fetchWithAuth(`/api/users/me/mcp-api-keys/${keyId}`, {
    method: 'DELETE',
  })
}
