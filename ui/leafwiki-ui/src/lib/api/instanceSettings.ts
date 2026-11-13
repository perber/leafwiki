import type { ContentWidth } from '@/lib/contentWidth'
import { fetchWithAuth } from './auth'

const PUBLIC_ACCESS_URL = '/api/admin/settings/public-access'
const TOC_DISPLAY_URL = '/api/admin/settings/toc-display'
const CONTENT_WIDTH_URL = '/api/admin/settings/content-width'

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

export type ContentWidthResponse = {
  contentWidth: ContentWidth
}

/**
 * Set the instance-wide page content width. Rejects with an
 * ApiLocalizedError carrying code `page_display_invalid_content_width`
 * (HTTP 400) for a value the server doesn't know.
 */
export async function setContentWidth(
  contentWidth: ContentWidth,
): Promise<ContentWidthResponse> {
  return await fetchWithAuth<ContentWidthResponse>(CONTENT_WIDTH_URL, {
    method: 'PUT',
    body: JSON.stringify({ contentWidth }),
  })
}
