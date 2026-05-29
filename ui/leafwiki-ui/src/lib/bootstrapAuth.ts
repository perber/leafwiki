import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { useEffect } from 'react'
import { ApiError, ensureRefresh, fetchMe } from './api/auth'

export function useBootstrapAuth(enabled = true) {
  const setUser = useSessionStore((s) => s.setUser)
  const setRefreshing = useSessionStore((s) => s.setRefreshing)
  const httpRemoteUserEnabled = useConfigStore((s) => s.httpRemoteUserEnabled)

  useEffect(() => {
    if (!enabled) {
      setRefreshing(false)
      return
    }
    let cancelled = false

    ;(async () => {
      setRefreshing(true)
      try {
        if (httpRemoteUserEnabled) {
          // Proxy manages the session — resolve the current user via /api/auth/me.
          // Token refresh does not apply in this mode.
          const user = await fetchMe()
          if (!cancelled) setUser(user)
        } else {
          await ensureRefresh()
        }
      } catch (err) {
        // Only clear the session for explicit auth failures (4xx).
        // Server errors (5xx) and network failures must not log the user out —
        // the backend returns 200 for both authenticated and unauthenticated
        // states, so a throw always indicates a genuine server problem.
        const isAuthFailure =
          err instanceof ApiError && err.status >= 400 && err.status < 500
        if (!cancelled && isAuthFailure) setUser(null)
        console.debug('[bootstrapAuth] Auth bootstrap failed:', err)
      } finally {
        if (!cancelled) setRefreshing(false)
      }
    })()

    return () => {
      cancelled = true
    }
  }, [setUser, setRefreshing, enabled, httpRemoteUserEnabled])
}
