// components/RoleGuard.tsx

import { hasRole } from '@/lib/roles'
import { useSessionStore } from '@/stores/session'
import { ReactNode } from 'react'

type Props = {
  roles: string[]
  children: ReactNode
}

export function RoleGuard({ roles, children }: Props) {
  const user = useSessionStore((state) => state.user)

  if (!hasRole(user?.role, roles)) return null

  return <>{children}</>
}
