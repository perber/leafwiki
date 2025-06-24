import { useAuthStore } from '@/stores/auth'
import { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { toast } from 'sonner'

type Props = {
  children: ReactNode
}

export default function RequireAuth({ children }: Props) {
  const token = useAuthStore((state) => state.token)
  const isRefreshing = useAuthStore((state) => state.isRefreshing)

  if (!token && !isRefreshing) {
    return <Navigate to="/login" replace />
  }

  if (!token && isRefreshing) {
    toast.info('Refreshing session, please wait...')
    return null // Return a loading state while refreshing
  }

  return <>{children}</>
}
