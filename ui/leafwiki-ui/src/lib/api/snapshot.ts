import { API_BASE_URL } from '../config'
import { fetchWithAuth } from './auth'

const SNAPSHOT_STATUS_URL = '/api/admin/snapshot/status'
const SNAPSHOT_LIST_URL = '/api/admin/snapshot'
const SNAPSHOT_TRIGGER_URL = '/api/admin/snapshot'

export interface SnapshotEntry {
  id: string
  createdAt: string
  sizeBytes: number
}

export interface SnapshotStatusResponse {
  enabled: boolean
  retentionCount?: number
  status?: {
    isRunning: boolean
    lastSnapshotAt: string | null
    lastError: string
    lastPruneError?: string
  }
}

export async function fetchSnapshotStatus(): Promise<SnapshotStatusResponse> {
  return await fetchWithAuth<SnapshotStatusResponse>(SNAPSHOT_STATUS_URL, {})
}

export async function fetchSnapshots(): Promise<SnapshotEntry[]> {
  const res = await fetchWithAuth<{ snapshots: SnapshotEntry[] }>(
    SNAPSHOT_LIST_URL,
    {},
  )
  return res.snapshots ?? []
}

export async function triggerSnapshot(): Promise<void> {
  await fetchWithAuth(SNAPSHOT_TRIGGER_URL, {
    method: 'POST',
  })
}

export async function deleteSnapshot(id: string): Promise<void> {
  await fetchWithAuth(`/api/admin/snapshot/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

// Browser-native download via <a href> — cookies carry auth, and GET is
// exempt from CSRF (see internal/http/middleware/security/csrf.go), so no
// need to route the zip bytes through fetchWithAuth.
export function snapshotDownloadUrl(id: string): string {
  return `${API_BASE_URL}/api/admin/snapshot/${encodeURIComponent(id)}/download`
}
