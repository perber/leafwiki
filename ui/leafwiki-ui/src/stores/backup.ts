import { create } from 'zustand'
import {
  fetchBackupStatus,
  triggerBackupPush,
  BackupStatusResponse,
} from '@/lib/api/backup'

interface BackupState {
  enabled: boolean
  lastBackupAt: string | null
  lastError: string
  needsIntervention: boolean
  conflictDetails: string
  isLoading: boolean
  isPolling: boolean
  statusError: string
  loadStatus: () => Promise<void>
  triggerPush: () => Promise<void>
  startPolling: () => void
  stopPolling: () => void
}

export const useBackupStore = create<BackupState>((set, get) => ({
  enabled: false,
  lastBackupAt: null,
  lastError: '',
  needsIntervention: false,
  conflictDetails: '',
  isLoading: false,
  isPolling: false,
  statusError: '',

  loadStatus: async () => {
    set({ isLoading: true })
    try {
      const data: BackupStatusResponse = await fetchBackupStatus()
      set({
        enabled: data.enabled,
        lastBackupAt: data.status?.lastBackupAt ?? null,
        lastError: data.status?.lastError ?? '',
        needsIntervention: data.status?.needsIntervention ?? false,
        conflictDetails: data.status?.conflictDetails ?? '',
        isLoading: false,
      })
    } catch {
      set({ isLoading: false, statusError: 'Failed to load backup status' })
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
