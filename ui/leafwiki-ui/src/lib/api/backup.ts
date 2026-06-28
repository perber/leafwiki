import { fetchWithAuth } from './auth'

const BACKUP_ALERT_URL = '/api/backup/alert'
const BACKUP_STATUS_URL = '/api/admin/backup/status'
const BACKUP_PUSH_URL = '/api/admin/backup/push'

export interface BackupStatusResponse {
  enabled: boolean
  status?: {
    lastBackupAt: string | null
    lastError: string
    needsIntervention: boolean
    conflictDetails: string
  }
}

export async function fetchBackupStatus(): Promise<BackupStatusResponse> {
  const res = await fetchWithAuth(BACKUP_STATUS_URL, {
    credentials: 'include',
  })
  return res as BackupStatusResponse
}

export async function fetchBackupAlert(): Promise<{
  needsIntervention: boolean
}> {
  const res = await fetchWithAuth(BACKUP_ALERT_URL, { credentials: 'include' })
  return res as { needsIntervention: boolean }
}

export async function triggerBackupPush(): Promise<void> {
  await fetchWithAuth(BACKUP_PUSH_URL, {
    method: 'POST',
    credentials: 'include',
  })
}
