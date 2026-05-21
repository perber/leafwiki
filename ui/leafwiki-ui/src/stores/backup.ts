import { create } from 'zustand'
import { fetchBackupStatus, triggerBackupPush, BackupStatusResponse } from '@/lib/api/backup'

interface BackupState {
  enabled: boolean
  lastBackupAt: string | null
  lastError: string
  isLoading: boolean
  isPolling: boolean
  loadStatus: () => Promise<void>
  triggerPush: () => Promise<void>
  startPolling: () => void
  stopPolling: () => void
}

export const useBackupStore = create<BackupState>((set, get) => ({
  enabled: false,
  lastBackupAt: null,
  lastError: '',
  isLoading: false,
  isPolling: false,

  loadStatus: async () => {
    set({ isLoading: true })
    try {
      const data: BackupStatusResponse = await fetchBackupStatus()
      set({
        enabled: data.enabled,
        lastBackupAt: data.status?.lastBackupAt ?? null,
        lastError: data.status?.lastError ?? '',
        isLoading: false,
      })
    } catch {
      set({ isLoading: false })
    }
  },

  triggerPush: async () => {
    await triggerBackupPush()
    get().startPolling()
  },

  startPolling: () => {
    set({ isPolling: true })
  },

  stopPolling: () => {
    set({ isPolling: false })
  },
}))