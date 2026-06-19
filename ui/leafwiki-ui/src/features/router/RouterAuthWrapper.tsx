import { ErrorBoundary } from '@/components/ErrorBoundary'
import RequireAuth from '@/features/auth/RequireAuth'
import AppLayout from '@/layout/AppLayout'
import { useLocation } from 'react-router-dom'

export default function AuthWrapper({
  children,
}: {
  children: React.ReactNode
}) {
  const { pathname } = useLocation()
  return (
    <ErrorBoundary resetKey={pathname}>
      <RequireAuth>
        <AppLayout>{children}</AppLayout>
      </RequireAuth>
    </ErrorBoundary>
  )
}
