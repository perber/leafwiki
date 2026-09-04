import { create } from 'zustand'
import {
  BackupConfig,
  BackupConfigInput,
  BackupStatusResponse,
  disableBackup,
  fetchBackupConfig,
  fetchBackupStatus,
  saveBackupConfig,
  testBackupConfig,
  triggerBackupPush,
  triggerForcePush,
} from '@/lib/api/backup'
import { mapApiError } from '@/lib/api/errors'
import i18next from '@/lib/i18n'

const DEFAULT_INTERVAL_MINUTES = 60

interface BackupState {
  enabled: boolean
  lastBackupAt: string | null
  lastError: string
  needsIntervention: boolean
  conflictDetails: string
  isLoading: boolean
  isPolling: boolean
  pollingFromAt: string | null
  statusError: string
  loadStatus: () => Promise<void>
  triggerPush: () => Promise<void>
  forcePush: () => Promise<void>
  startPolling: () => void
  stopPolling: () => void

  // Settings-mode configuration.
  envManaged: boolean
  bootError: string
  configAvailable: boolean
  encryptionKeyAvailable: boolean
  minIntervalMinutes: number
  maxIntervalMinutes: number
  config: BackupConfig | null
  configLoading: boolean
  configError: string
  loadConfig: () => Promise<void>
  testConfig: (input: BackupConfigInput) => Promise<void>
  saveConfig: (input: BackupConfigInput) => Promise<void>
  disable: () => Promise<void>
}

export const useBackupStore = create<BackupState>((set, get) => ({
  enabled: false,
  lastBackupAt: null,
  lastError: '',
  needsIntervention: false,
  conflictDetails: '',
  isLoading: false,
  isPolling: false,
  pollingFromAt: null,
  statusError: '',

  envManaged: false,
  bootError: '',
  configAvailable: false,
  encryptionKeyAvailable: true,
  minIntervalMinutes: 2,
  maxIntervalMinutes: 1440,
  config: null,
  configLoading: false,
  configError: '',

  loadStatus: async () => {
    if (!get().isPolling) {
      set({ isLoading: true })
    }
    try {
      const data: BackupStatusResponse = await fetchBackupStatus()
      set({
        enabled: data.enabled,
        envManaged: data.envManaged ?? get().envManaged,
        bootError: data.bootError ?? '',
        lastBackupAt: data.status?.lastBackupAt ?? null,
        lastError: data.status?.lastError ?? '',
        needsIntervention: data.status?.needsIntervention ?? false,
        conflictDetails: data.status?.conflictDetails ?? '',
        isLoading: false,
        statusError: '',
      })
    } catch {
      set({
        isLoading: false,
        statusError: i18next.t('loadStatusErrorFallback', { ns: 'backup' }),
      })
    }
  },

  triggerPush: async () => {
    await triggerBackupPush()
    get().startPolling()
  },

  forcePush: async () => {
    await triggerForcePush()
    await get().loadStatus()
  },

  startPolling: () => {
    set({ isPolling: true, pollingFromAt: get().lastBackupAt })
  },

  stopPolling: () => {
    set({ isPolling: false, pollingFromAt: null })
  },

  loadConfig: async () => {
    set({ configLoading: true, configError: '' })
    try {
      const res = await fetchBackupConfig()
      set({
        configLoading: false,
        configAvailable: res.available,
        envManaged: res.envManaged ?? false,
        enabled: res.enabled ?? get().enabled,
        encryptionKeyAvailable: res.encryptionKeyAvailable ?? true,
        minIntervalMinutes: res.minIntervalMinutes ?? 2,
        maxIntervalMinutes: res.maxIntervalMinutes ?? 1440,
        bootError: res.bootError ?? '',
        config: res.config ?? null,
      })
    } catch (err) {
      set({
        configLoading: false,
        configError: mapApiError(
          err,
          i18next.t('config.loadError', { ns: 'backup' }),
        ).message,
      })
    }
  },

  testConfig: async (input: BackupConfigInput) => {
    await testBackupConfig(input)
  },

  saveConfig: async (input: BackupConfigInput) => {
    const res = await saveBackupConfig(input)
    set({
      configAvailable: res.available,
      enabled: res.enabled ?? true,
      encryptionKeyAvailable: res.encryptionKeyAvailable ?? true,
      bootError: res.bootError ?? '',
      config: res.config ?? get().config,
    })
    await get().loadStatus()
  },

  disable: async () => {
    await disableBackup()
    set({ enabled: false })
    await get().loadConfig()
    await get().loadStatus()
  },
}))

export { DEFAULT_INTERVAL_MINUTES }
