import { useSessionStore } from '@/stores/session'
import { useEffect } from 'react'
import { ensureRefresh } from './api/auth'

export function useBootstrapAuth(enabled = true) {
  const setUser = useSessionStore((s) => s.setUser)
  const setRefreshing = useSessionStore((s) => s.setRefreshing)

  useEffect(() => {
    if (!enabled) {
      setRefreshing(false)
      return
    }
    let cancelled = false

    ;(async () => {
      setRefreshing(true)
      try {
        await ensureRefresh()
      } catch (err) {
        // Clear user state when refresh fails to prevent stale data
        if (!cancelled) setUser(null)
        console.debug('[bootstrapAuth] Refresh token failed:', err)
      } finally {
        if (!cancelled) setRefreshing(false)
      }
    })()

    return () => {
      cancelled = true
    }
  }, [setUser, setRefreshing, enabled])
}
