import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'

export function useIsReadOnly() {
  const user = useSessionStore((s) => s.user)
  const publicAccess = useConfigStore((s) => s.publicAccess)
  // Not logged in, check public access setting
  if (!user) {
    return publicAccess
  }

  // viewer role -> only read
  if (user.role === 'viewer') {
    return true
  }

  // admin and editor roles -> read and write
  return false
}
