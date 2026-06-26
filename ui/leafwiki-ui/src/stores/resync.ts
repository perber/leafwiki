import { create } from 'zustand'
import { getResyncStatus, triggerResync } from '@/lib/api/resync'

const POLL_INTERVAL_MS = 800
const POLL_ERROR_LIMIT = 3

interface ResyncState {
  isLoading: boolean
  phase: string | null
  trigger: () => Promise<void>
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

export const useResyncStore = create<ResyncState>((set) => ({
  isLoading: false,
  phase: null,

  trigger: async () => {
    // Set loading before the POST so the button is disabled immediately,
    // preventing a second concurrent trigger during the network round-trip.
    set({ isLoading: true, phase: null })
    try {
      await triggerResync()
    } catch (err) {
      set({ isLoading: false })
      throw err
    }

    // Poll until the job reports done or a network error is unrecoverable.
    let consecutiveErrors = 0
    for (;;) {
      await sleep(POLL_INTERVAL_MS)

      let status: Awaited<ReturnType<typeof getResyncStatus>>
      try {
        status = await getResyncStatus()
        consecutiveErrors = 0
      } catch (err) {
        consecutiveErrors++
        if (consecutiveErrors >= POLL_ERROR_LIMIT) {
          set({ isLoading: false, phase: null })
          throw err
        }
        continue
      }

      // || null converts "" (Go's zero-value for omitted phase) to null.
      set({ phase: status.phase || null })

      if (status.done) {
        set({ isLoading: false, phase: null })
        // Throw application error outside the try/catch so it is not
        // mistaken for a network error and does not increment consecutiveErrors.
        if (status.error) {
          throw new Error(status.error)
        }
        return
      }
      // running=false without done=true means the job was lost (server restart).
      if (!status.running) {
        set({ isLoading: false, phase: null })
        throw new Error('Sync job lost — server may have restarted')
      }
    }
  },
}))
