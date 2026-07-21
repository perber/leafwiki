import { create } from 'zustand'
import i18next from '@/lib/i18n'
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
  // False when the tail-end resync's completion could not be confirmed (the
  // poll lost track of the job or kept erroring). The restore itself still
  // succeeded at this point — only the search/links/tags rebuild status is
  // unknown — so this is a soft signal for the UI to caveat, not an error to
  // throw and have the caller mistake for the restore itself having failed.
  resyncConfirmed: boolean
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
  resyncConfirmed: true,

  trigger: async (id: string) => {
    set({
      isLoading: true,
      phase: null,
      isResyncPhase: false,
      needsIntervention: false,
      versionWarning: null,
      resyncConfirmed: true,
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
        throw new Error(i18next.t('errors.jobLost', { ns: 'restore' }))
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
// triggered server-side finishes. Never throws: the restore itself already
// succeeded by the time this runs, so a lost job or repeated poll errors
// here are reported via resyncConfirmed=false (a caveat), not as a failure
// of the overall restore.
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
      if (status.done) {
        set({ isLoading: false, phase: null, isResyncPhase: false })
        return
      }
      if (!status.running) {
        // Job lost (e.g. server restarted) before ever reporting done.
        set({
          isLoading: false,
          phase: null,
          isResyncPhase: false,
          resyncConfirmed: false,
        })
        return
      }
    } catch {
      consecutiveErrors++
      if (consecutiveErrors >= POLL_ERROR_LIMIT) {
        set({
          isLoading: false,
          phase: null,
          isResyncPhase: false,
          resyncConfirmed: false,
        })
        return
      }
    }
  }
}
