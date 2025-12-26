import { useSessionStore } from '@/stores/session'
import { useEffect } from 'react'
import { API_BASE_URL } from './config'

export function useBootstrapAuth() {
  const setUser = useSessionStore((s) => s.setUser)
  const setRefreshing = useSessionStore((s) => s.setRefreshing)

  useEffect(() => {
    let cancelled = false

    ;(async () => {
      setRefreshing(true)
      try {
        const res = await fetch(`${API_BASE_URL}/api/auth/refresh-token`, {
          method: 'POST',
          credentials: 'include',
        })

        if (!res.ok) {
          return
        }

        const data = await res.json()
        if (!cancelled) setUser(data.user)
      } catch (err) {
        // ignore
        // When refresh fails, user is not logged in, so we do nothing
        console.debug('[bootstrapAuth] Refresh token failed:', err)
      } finally {
        if (!cancelled) setRefreshing(false)
      }
    })()

    return () => {
      cancelled = true
    }
  }, [setUser, setRefreshing])
}
