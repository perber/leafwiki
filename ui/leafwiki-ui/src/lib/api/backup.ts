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
  branch: string
  authorName: string
  authorEmail: string
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
  const res = await fetchWithAuth(BACKUP_STATUS_URL, {
    credentials: 'include',
  })
  return res as BackupStatusResponse
}

export async function fetchBackupAlert(): Promise<{
  needsIntervention: boolean
  hasError: boolean
}> {
  const res = await fetchWithAuth(BACKUP_ALERT_URL, { credentials: 'include' })
  return res as { needsIntervention: boolean; hasError: boolean }
}

export async function triggerBackupPush(): Promise<void> {
  await fetchWithAuth(BACKUP_PUSH_URL, {
    method: 'POST',
    credentials: 'include',
  })
}

export async function triggerForcePush(): Promise<void> {
  await fetchWithAuth(BACKUP_FORCE_PUSH_URL, {
    method: 'POST',
    credentials: 'include',
  })
}

export async function triggerPull(): Promise<void> {
  await fetchWithAuth(BACKUP_PULL_URL, {
    method: 'POST',
    credentials: 'include',
  })
}

export async function fetchBackupConfig(): Promise<BackupConfigResponse> {
  const res = await fetchWithAuth(BACKUP_CONFIG_URL, { credentials: 'include' })
  return res as BackupConfigResponse
}

export async function saveBackupConfig(
  input: BackupConfigInput,
): Promise<BackupConfigResponse> {
  const res = await fetchWithAuth(BACKUP_CONFIG_URL, {
    method: 'POST',
    credentials: 'include',
    body: JSON.stringify(input),
  })
  return res as BackupConfigResponse
}

export async function testBackupConfig(
  input: BackupConfigInput,
): Promise<void> {
  await fetchWithAuth(BACKUP_CONFIG_TEST_URL, {
    method: 'POST',
    credentials: 'include',
    body: JSON.stringify(input),
  })
}

export async function disableBackup(): Promise<void> {
  await fetchWithAuth(BACKUP_DISABLE_URL, {
    method: 'POST',
    credentials: 'include',
  })
}
