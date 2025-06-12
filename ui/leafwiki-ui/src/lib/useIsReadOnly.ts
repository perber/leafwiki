import { useAuthStore } from '@/stores/auth'
import { usePublicAccessStore } from '@/stores/publicAccess'

export function useIsReadOnly() {
  const user = useAuthStore((s) => s.user)
  const publicAccess = usePublicAccessStore((s) => s.publicAccess)
  return publicAccess && !user
}
