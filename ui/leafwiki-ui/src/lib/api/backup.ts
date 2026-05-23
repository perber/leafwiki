import { API_BASE_URL } from '../config'
import { fetchWithAuth } from './auth'

const BACKUP_STATUS_URL = '/api/admin/backup/status'
const BACKUP_PUSH_URL = '/api/admin/backup/push'

export interface BackupStatusResponse {
  enabled: boolean
  status?: {
    lastBackupAt: string | null
    lastError: string
  }
}

export async function fetchBackupStatus(): Promise<BackupStatusResponse> {
  const res = await fetchWithAuth(`${API_BASE_URL}${BACKUP_STATUS_URL}`, {
    credentials: 'include',
  })
  return res as BackupStatusResponse
}

export async function triggerBackupPush(): Promise<void> {
  await fetchWithAuth(BACKUP_PUSH_URL, {
    method: 'POST',
  })
}
