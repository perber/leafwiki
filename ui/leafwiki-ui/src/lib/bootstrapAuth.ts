import { useSessionStore } from '@/stores/session'
import { useEffect } from 'react'
import { API_BASE_URL } from './config'

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
        const res = await fetch(`${API_BASE_URL}/api/auth/refresh-token`, {
          method: 'POST',
          credentials: 'include',
        })

        if (!res.ok) {
          // Clear user state when refresh fails to prevent stale data
          if (!cancelled) setUser(null)
          return
        }

        const data = await res.json()
        if (!cancelled) setUser(data.user)
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
