import { withBasePath } from '../routePath'
import { fetchWithAuth } from './auth'

export async function uploadAvatar(file: File): Promise<void> {
  const formData = new FormData()
  formData.append('file', file)

  await fetchWithAuth('/api/user/avatar', {
    method: 'POST',
    body: formData,
    headers: {}, // Let browser set Content-Type for FormData
  })
}

export async function deleteAvatar(): Promise<void> {
  await fetchWithAuth('/api/user/avatar', {
    method: 'DELETE',
  })
}

/**
 * Builds the public, unauthenticated URL for a user's avatar image.
 * `version` is a cache-busting query param, mirroring stores/branding.ts's
 * logoVersion/faviconVersion idiom — bump it after every upload/delete so a
 * stale browser-cached image isn't shown after the underlying file changes.
 */
export function avatarUrl(userId: string, version?: number): string {
  const path = withBasePath(`/avatars/${userId}`)
  return version !== undefined ? `${path}?v=${version}` : path
}
