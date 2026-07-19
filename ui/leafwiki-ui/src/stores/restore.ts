import { create } from 'zustand'
import {
  triggerRestore,
  getRestoreStatus,
  triggerSelfRestart,
  RestoreStatus,
} from '@/lib/api/restore'
import { getResyncStatus } from '@/lib/api/resync'

const POLL_INTERVAL_MS = 800
const POLL_ERROR_LIMIT = 3

interface RestoreState {
  isLoading: boolean
  phase: string | null
  // True once the restore job itself is done and this store has switched to
  // polling the existing resync status endpoint for the tail end (rebuilding
  // search/links/tags) — the backend already triggered that resync, this
  // store never issues its own resync trigger.
  isResyncPhase: boolean
  needsIntervention: boolean
  versionWarning: string | null
  trigger: (id: string) => Promise<void>
  selfRestart: () => Promise<void>
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

export const useRestoreStore = create<RestoreState>((set) => ({
  isLoading: false,
  phase: null,
  isResyncPhase: false,
  needsIntervention: false,
  versionWarning: null,

  trigger: async (id: string) => {
    set({
      isLoading: true,
      phase: null,
      isResyncPhase: false,
      needsIntervention: false,
      versionWarning: null,
    })
    try {
      await triggerRestore(id)
    } catch (err) {
      set({ isLoading: false })
      throw err
    }

    let consecutiveErrors = 0
    for (;;) {
      await sleep(POLL_INTERVAL_MS)

      let status: RestoreStatus
      try {
        status = await getRestoreStatus()
        consecutiveErrors = 0
      } catch (err) {
        consecutiveErrors++
        if (consecutiveErrors >= POLL_ERROR_LIMIT) {
          set({ isLoading: false, phase: null })
          throw err
        }
        continue
      }

      set({
        phase: status.phase || null,
        versionWarning: status.versionWarning || null,
      })

      if (status.done) {
        if (status.needsIntervention) {
          set({ isLoading: false, needsIntervention: true })
          return
        }
        if (status.error) {
          set({ isLoading: false, phase: null })
          throw new Error(status.error)
        }
        set({ isResyncPhase: true })
        await pollResyncTail(set)
        return
      }
      if (!status.running) {
        set({ isLoading: false, phase: null })
        throw new Error('Restore job lost — server may have restarted')
      }
    }
  },

  selfRestart: async () => {
    try {
      await triggerSelfRestart()
    } catch {
      // Expected: the connection drops when the server process replaces
      // itself mid-request. Not a real failure — see triggerSelfRestart's
      // doc comment in lib/api/restore.ts.
    }
  },
}))

// pollResyncTail polls the existing resync status endpoint (no new endpoint,
// per ADR-0004's polling pattern) until the resync the restore already
// triggered server-side finishes.
async function pollResyncTail(
  set: (partial: Partial<RestoreState>) => void,
): Promise<void> {
  let consecutiveErrors = 0
  for (;;) {
    await sleep(POLL_INTERVAL_MS)
    try {
      const status = await getResyncStatus()
      consecutiveErrors = 0
      set({ phase: status.phase || null })
      if (status.done || !status.running) {
        set({ isLoading: false, phase: null, isResyncPhase: false })
        return
      }
    } catch {
      consecutiveErrors++
      if (consecutiveErrors >= POLL_ERROR_LIMIT) {
        set({ isLoading: false, phase: null, isResyncPhase: false })
        return
      }
    }
  }
}
