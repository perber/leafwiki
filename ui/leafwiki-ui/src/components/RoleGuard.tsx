// components/RoleGuard.tsx

import { useAuthStore } from '@/stores/auth'
import { ReactNode } from 'react'

type Props = {
  roles: string[]
  children: ReactNode
}

export function RoleGuard({ roles, children }: Props) {
  const user = useAuthStore((state) => state.user)

  if (!user) return null
  if (!roles.includes(user.role)) return null

  return <>{children}</>
}
