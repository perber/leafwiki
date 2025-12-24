// components/RoleGuard.tsx

import { useSessionStore } from '@/stores/session'
import { ReactNode } from 'react'

type Props = {
  roles: string[]
  children: ReactNode
}

export function RoleGuard({ roles, children }: Props) {
  const user = useSessionStore((state) => state.user)

  if (!user) return null
  if (!roles.includes(user.role)) return null

  return <>{children}</>
}
