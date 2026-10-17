import { fetchWithAuth } from './auth'

const BACKUP_ALERT_URL = '/api/backup/alert'
const BACKUP_STATUS_URL = '/api/admin/backup/status'
const BACKUP_PUSH_URL = '/api/admin/backup/push'
const BACKUP_FORCE_PUSH_URL = '/api/admin/backup/force-push'
const BACKUP_PULL_URL = '/api/admin/backup/pull'
const BACKUP_CONFIG_URL = '/api/admin/backup/config'
const BACKUP_CONFIG_TEST_URL = '/api/admin/backup/config/test'
const BACKUP_DISABLE_URL = '/api/admin/backup/disable'

export interface BackupStatusResponse {
  enabled: boolean
  envManaged?: boolean
  bootError?: string
  status?: {
    lastBackupAt: string | null
    lastError: string
    needsIntervention: boolean
    conflictDetails: string
  }
}

export type BackupAuthMode = 'ssh' | 'https' | ''

export interface BackupConfig {
  remoteUrl: string
  // Repository-relative directory content is committed under; '' = repository
  // top level. Useful when the backup remote is a monorepo.
  path: string
  branch: string
  authorName: string
  authorEmail: string
  // Transport derived by the server from the remote URL scheme. The form does
  // not use it — it derives the same thing from the URL — but it is part of the
  // GET /backup/config response.
  authMode: BackupAuthMode
  sshKeyPath: string
  sshKnownHostsPath: string
  httpUsername: string
  hasSshKey: boolean
  hasHttpPassword: boolean
  intervalMinutes: number
}

export interface BackupConfigResponse {
  available: boolean
  envManaged?: boolean
  enabled?: boolean
  encryptionKeyAvailable?: boolean
  minIntervalMinutes?: number
  maxIntervalMinutes?: number
  config?: BackupConfig
  bootError?: string
}

// Fields the form submits. Empty sshKey / httpPassword mean "keep the stored value".
export interface BackupConfigInput {
  remoteUrl: string
  path: string
  branch: string
  authorName: string
  authorEmail: string
  sshKey: string
  sshKeyPath: string
  sshKnownHostsPath: string
  httpUsername: string
  httpPassword: string
  intervalMinutes: number
}

export async function fetchBackupStatus(): Promise<BackupStatusResponse> {
  return await fetchWithAuth<BackupStatusResponse>(BACKUP_STATUS_URL)
}

export async function fetchBackupAlert(): Promise<{
  needsIntervention: boolean
  hasError: boolean
}> {
  return await fetchWithAuth<{
    needsIntervention: boolean
    hasError: boolean
  }>(BACKUP_ALERT_URL)
}

export async function triggerBackupPush(): Promise<void> {
  await fetchWithAuth(BACKUP_PUSH_URL, {
    method: 'POST',
  })
}

export async function triggerForcePush(): Promise<void> {
  await fetchWithAuth(BACKUP_FORCE_PUSH_URL, {
    method: 'POST',
  })
}

export async function triggerPull(): Promise<void> {
  await fetchWithAuth(BACKUP_PULL_URL, {
    method: 'POST',
  })
}

export async function fetchBackupConfig(): Promise<BackupConfigResponse> {
  return await fetchWithAuth<BackupConfigResponse>(BACKUP_CONFIG_URL)
}

export async function saveBackupConfig(
  input: BackupConfigInput,
): Promise<BackupConfigResponse> {
  return await fetchWithAuth<BackupConfigResponse>(BACKUP_CONFIG_URL, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function testBackupConfig(
  input: BackupConfigInput,
): Promise<void> {
  await fetchWithAuth(BACKUP_CONFIG_TEST_URL, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export async function disableBackup(): Promise<void> {
  await fetchWithAuth(BACKUP_DISABLE_URL, {
    method: 'POST',
  })
}
