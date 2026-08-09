import { create } from 'zustand'
import i18next from '@/lib/i18n'
import {
  fetchSnapshotStatus,
  fetchSnapshots,
  triggerSnapshot,
  deleteSnapshot,
  SnapshotEntry,
} from '@/lib/api/snapshot'
import { sleep } from '@/lib/sleep'

const POLL_INTERVAL_MS = 800
const POLL_ERROR_LIMIT = 3

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

export const useSnapshotStore = create<SnapshotState>((set, get) => {
  // The trigger POST only starts the job (202 Accepted); the snapshot itself
  // finishes asynchronously on the server. Poll status until it's done
  // before reloading the list — otherwise a slow snapshot (large wiki, busy
  // disk) can outlast the single immediate status check right after the
  // POST, leaving the list stuck showing stale data with nothing left to
  // ever refresh it.
  async function pollUntilNotRunning(): Promise<void> {
    let consecutiveErrors = 0
    while (get().isRunning) {
      await sleep(POLL_INTERVAL_MS)
      try {
        const data = await fetchSnapshotStatus()
        set({
          isRunning: data.status?.isRunning ?? false,
          lastSnapshotAt: data.status?.lastSnapshotAt ?? null,
          lastError: data.status?.lastError ?? '',
          lastPruneError: data.status?.lastPruneError ?? '',
        })
        consecutiveErrors = 0
      } catch {
        consecutiveErrors++
        if (consecutiveErrors >= POLL_ERROR_LIMIT) {
          return
        }
      }
    }
  }

  return {
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
      try {
        await triggerSnapshot()
      } finally {
        // Reload even on failure (e.g. 409 because a run is already in
        // progress) so the UI reflects current server state instead of
        // staying stale behind the error.
        await get().loadStatus()
        await pollUntilNotRunning()
        await get().loadList()
      }
    },

    remove: async (id: string) => {
      await deleteSnapshot(id)
      await get().loadList()
    },
  }
})
