import type { PageDisplay, PageDisplayPatch } from '@/lib/pageDisplay'
import { fetchWithAuth } from './auth'

const PUBLIC_ACCESS_URL = '/api/admin/settings/public-access'
const PAGE_DISPLAY_URL = '/api/admin/settings/display'

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

/**
 * Change any of the instance-wide page display settings. The server applies
 * the given fields in one write and replies with all display settings.
 * Rejects with an ApiLocalizedError carrying code
 * `page_display_invalid_content_width` (HTTP 400) for a width the server
 * doesn't know; nothing is changed then.
 */
export async function updatePageDisplay(
  patch: PageDisplayPatch,
): Promise<PageDisplay> {
  return await fetchWithAuth<PageDisplay>(PAGE_DISPLAY_URL, {
    method: 'PUT',
    body: JSON.stringify(patch),
  })
}
