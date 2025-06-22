import RequireAuth from '@/features/auth/RequireAuth'
import AppLayout from '@/layout/AppLayout'
import { PageToolbarProvider } from '../../components/PageToolbarProvider'

export default function AuthWrapper({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <RequireAuth>
      <PageToolbarProvider>
        <AppLayout>{children}</AppLayout>
      </PageToolbarProvider>
    </RequireAuth>
  )
}
