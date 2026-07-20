import { fetchWithAuth } from './auth'

export type ApiKeyRole = 'admin' | 'editor' | 'viewer'

export type ApiKey = {
  id: string
  name: string
  userId: string
  prefix: string
  role: ApiKeyRole
  createdBy: string
  createdAt: string
  expiresAt?: string
  lastUsedAt?: string
  revokedAt?: string
}

export type CreateApiKeyInput = {
  name: string
  userId: string
  role?: ApiKeyRole
  expiresAt?: string
}

export type CreateApiKeyResult = {
  key: ApiKey
  secret: string
}

export async function getApiKeys(): Promise<ApiKey[]> {
  return (await fetchWithAuth('/api/api-keys')) as ApiKey[]
}

export async function createApiKey(
  input: CreateApiKeyInput,
): Promise<CreateApiKeyResult> {
  return (await fetchWithAuth('/api/api-keys', {
    method: 'POST',
    body: JSON.stringify(input),
  })) as CreateApiKeyResult
}

export async function deleteApiKey(id: string) {
  return await fetchWithAuth(`/api/api-keys/${id}`, {
    method: 'DELETE',
  })
}
