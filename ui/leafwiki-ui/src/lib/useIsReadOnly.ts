import { usePublicAccessStore } from '@/stores/publicAccess'
import { useSessionStore } from '@/stores/session'

export function useIsReadOnly() {
  const user = useSessionStore((s) => s.user)
  const publicAccess = usePublicAccessStore((s) => s.publicAccess)
  return publicAccess && !user
}
