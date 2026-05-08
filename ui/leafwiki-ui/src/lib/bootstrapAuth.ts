import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { useEffect } from 'react'
import { ensureRefresh, fetchMe } from './api/auth'

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
        if (!cancelled) setUser(null)
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
