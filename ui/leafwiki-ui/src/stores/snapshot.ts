import { create } from 'zustand'
import i18next from '@/lib/i18n'
import {
  fetchSnapshotStatus,
  fetchSnapshots,
  triggerSnapshot,
  deleteSnapshot,
  SnapshotEntry,
} from '@/lib/api/snapshot'

interface SnapshotState {
  enabled: boolean
  retentionCount: number
  isRunning: boolean
  lastSnapshotAt: string | null
  lastError: string
  lastPruneError: string
  snapshots: SnapshotEntry[]
  isLoading: boolean
  isListLoading: boolean
  statusError: string
  loadStatus: () => Promise<void>
  loadList: () => Promise<void>
  triggerNow: () => Promise<void>
  remove: (id: string) => Promise<void>
}

export const useSnapshotStore = create<SnapshotState>((set, get) => ({
  enabled: false,
  retentionCount: 0,
  isRunning: false,
  lastSnapshotAt: null,
  lastError: '',
  lastPruneError: '',
  snapshots: [],
  isLoading: false,
  isListLoading: false,
  statusError: '',

  loadStatus: async () => {
    set({ isLoading: true })
    try {
      const data = await fetchSnapshotStatus()
      set({
        enabled: data.enabled,
        retentionCount: data.retentionCount ?? 0,
        isRunning: data.status?.isRunning ?? false,
        lastSnapshotAt: data.status?.lastSnapshotAt ?? null,
        lastError: data.status?.lastError ?? '',
        lastPruneError: data.status?.lastPruneError ?? '',
        isLoading: false,
        statusError: '',
      })
    } catch (error) {
      console.error('Failed to load snapshot status', error)
      set({
        isLoading: false,
        statusError: i18next.t('statusLoadError', { ns: 'snapshot' }),
      })
    }
  },

  loadList: async () => {
    set({ isListLoading: true })
    try {
      const snapshots = await fetchSnapshots()
      set({ snapshots, isListLoading: false })
    } catch (error) {
      console.error('Failed to load snapshots', error)
      set({ isListLoading: false })
    }
  },

  triggerNow: async () => {
    await triggerSnapshot()
    await get().loadStatus()
    await get().loadList()
  },

  remove: async (id: string) => {
    await deleteSnapshot(id)
    await get().loadList()
  },
}))
