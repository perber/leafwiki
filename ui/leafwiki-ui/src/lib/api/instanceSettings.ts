import { fetchWithAuth } from './auth'

const PUBLIC_ACCESS_URL = '/api/admin/settings/public-access'

export type PublicAccessResponse = {
  enabled: boolean
}

/**
 * Toggle "public mode" (anonymous read access to every page) at runtime.
 * Rejects with an ApiLocalizedError carrying code `public_access_env_managed`
 * (HTTP 409) when the instance pins the flag via environment configuration.
 */
export async function setPublicAccess(
  enabled: boolean,
): Promise<PublicAccessResponse> {
  const res = await fetchWithAuth(PUBLIC_ACCESS_URL, {
    method: 'PUT',
    credentials: 'include',
    body: JSON.stringify({ enabled }),
  })
  return res as PublicAccessResponse
}
