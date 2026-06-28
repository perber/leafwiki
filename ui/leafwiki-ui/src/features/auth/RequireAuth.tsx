import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'

type Props = {
  children: ReactNode
}

export default function RequireAuth({ children }: Props) {
  const user = useSessionStore((state) => state.user)
  const isRefreshing = useSessionStore((state) => state.isRefreshing)
  const authDisabled = useConfigStore((state) => state.authDisabled)
  const location = useLocation()

  if (authDisabled) return <>{children}</>

  if (!user && !isRefreshing) {
    const redirectTo = `${location.pathname}${location.search}${location.hash}`
    return <Navigate to="/login" replace state={{ redirectTo }} />
  }

  if (!user && isRefreshing) {
    return null // Return a loading state while refreshing
  }

  return <>{children}</>
}
