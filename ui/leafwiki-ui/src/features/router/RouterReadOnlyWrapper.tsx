import { ErrorBoundary } from '@/components/ErrorBoundary'
import AppLayout from '@/layout/AppLayout'
import { useLocation } from 'react-router-dom'

export default function ReadOnlyWrapper({
  children,
}: {
  children: React.ReactNode
}) {
  const { pathname } = useLocation()
  return (
    <ErrorBoundary resetKey={pathname}>
      <AppLayout>{children}</AppLayout>
    </ErrorBoundary>
  )
}
