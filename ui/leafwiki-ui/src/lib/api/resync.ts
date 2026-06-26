import { fetchWithAuth } from './auth'

const RESYNC_URL = '/api/admin/resync'

export interface ResyncStatus {
  running: boolean
  phase: string | null
  done: boolean
  error?: string
}

export async function triggerResync(): Promise<void> {
  await fetchWithAuth(RESYNC_URL, {
    method: 'POST',
    credentials: 'include',
  })
}

export async function getResyncStatus(): Promise<ResyncStatus> {
  const res = await fetchWithAuth(`${RESYNC_URL}/status`, {
    credentials: 'include',
  })
  return res as ResyncStatus
}
