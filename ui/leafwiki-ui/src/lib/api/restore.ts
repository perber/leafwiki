import { fetchWithAuth } from './auth'

const RESTORE_URL = '/api/admin/restore'

export interface RestoreStatus {
  running: boolean
  phase: string | null
  done: boolean
  error?: string
  versionWarning?: string
  needsIntervention?: boolean
}

export async function triggerRestore(id: string): Promise<void> {
  await fetchWithAuth(`${RESTORE_URL}/${encodeURIComponent(id)}`, {
    method: 'POST',
    credentials: 'include',
  })
}

export async function getRestoreStatus(): Promise<RestoreStatus> {
  const res = await fetchWithAuth(`${RESTORE_URL}/status`, {
    credentials: 'include',
  })
  return res as RestoreStatus
}

// triggerSelfRestart's request is expected to never receive a clean
// response: the server process replaces itself (Unix) or exits (Windows)
// mid-handler, dropping the connection. Callers should treat any rejection
// from this call as an unremarkable, expected outcome — not a real failure —
// and move on to waiting for the server to come back up.
export async function triggerSelfRestart(): Promise<void> {
  await fetchWithAuth(`${RESTORE_URL}/self-restart`, {
    method: 'POST',
    credentials: 'include',
  })
}
