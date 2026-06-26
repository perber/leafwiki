import { fetchWithAuth } from './auth'

const RESYNC_URL = '/api/admin/resync'

export async function triggerResync(): Promise<void> {
  await fetchWithAuth(RESYNC_URL, {
    method: 'POST',
    credentials: 'include',
  })
}
