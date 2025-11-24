import RequireAuth from '@/features/auth/RequireAuth'
import AppLayout from '@/layout/AppLayout'

export default function AuthWrapper({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <RequireAuth>
      <AppLayout>{children}</AppLayout>
    </RequireAuth>
  )
}
