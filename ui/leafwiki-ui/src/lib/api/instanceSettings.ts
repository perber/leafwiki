import { fetchWithAuth } from './auth'

const PUBLIC_ACCESS_URL = '/api/admin/settings/public-access'
const TOC_DISPLAY_URL = '/api/admin/settings/toc-display'

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

export type TocDisplayResponse = {
  alwaysShow: boolean
}

/**
 * Toggle "always show table of contents" (show the TOC panel/dropdown
 * regardless of heading count) at runtime.
 */
export async function setAlwaysShowToc(
  alwaysShow: boolean,
): Promise<TocDisplayResponse> {
  const res = await fetchWithAuth(TOC_DISPLAY_URL, {
    method: 'PUT',
    credentials: 'include',
    body: JSON.stringify({ alwaysShow }),
  })
  return res as TocDisplayResponse
}
