import { useAuthStore } from '@/stores/auth'
import { useConfigStore } from '@/stores/config'

export function useIsReadOnly() {
  const user = useAuthStore((s) => s.user)
  const publicAccess = useConfigStore((s) => s.publicAccess)
  return publicAccess && !user
}
