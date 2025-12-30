import { useConfigStore } from '@/stores/config'
import { useSessionStore } from '@/stores/session'

export function useIsReadOnly() {
  const user = useSessionStore((s) => s.user)
  const publicAccess = useConfigStore((s) => s.publicAccess)
  return publicAccess && !user
}
