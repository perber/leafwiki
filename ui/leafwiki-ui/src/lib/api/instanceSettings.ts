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
  return await fetchWithAuth<PublicAccessResponse>(PUBLIC_ACCESS_URL, {
    method: 'PUT',
    body: JSON.stringify({ enabled }),
  })
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
  return await fetchWithAuth<TocDisplayResponse>(TOC_DISPLAY_URL, {
    method: 'PUT',
    body: JSON.stringify({ alwaysShow }),
  })
}
