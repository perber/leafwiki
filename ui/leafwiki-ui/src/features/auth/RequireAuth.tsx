import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'
import { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import ExternalRedirect from './ExternalRedirect'

type Props = {
  children: ReactNode
}

export default function RequireAuth({ children }: Props) {
  const user = useSessionStore((state) => state.user)
  const isRefreshing = useSessionStore((state) => state.isRefreshing)
  const authDisabled = useConfigStore((state) => state.authDisabled)
  const loginUrl = useConfigStore((state) => state.loginUrl)
  const location = useLocation()

  if (authDisabled) return <>{children}</>

  if (!user && !isRefreshing) {
    const redirectTo = `${location.pathname}${location.search}${location.hash}`
    if (loginUrl) {
      return <ExternalRedirect to={loginUrl} returnTo={redirectTo} />
    }
    return <Navigate to="/login" replace state={{ redirectTo }} />
  }

  if (!user && isRefreshing) {
    return null // Return a loading state while refreshing
  }

  return <>{children}</>
}
