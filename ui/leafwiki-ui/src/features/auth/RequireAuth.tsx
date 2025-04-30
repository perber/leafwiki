import { useAuthStore } from '@/stores/auth'
import { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'

type Props = {
  children: ReactNode
}

export default function RequireAuth({ children }: Props) {
  const token = useAuthStore((state) => state.token)
  const isRefreshing = useAuthStore((state) => state.isRefreshing)

  if (isRefreshing) return null

  if (!token) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}
