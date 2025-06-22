import AppLayout from '@/layout/AppLayout'
import { PageToolbarProvider } from '../../components/PageToolbarProvider'

export default function ReadOnlyWrapper({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <PageToolbarProvider>
      <AppLayout>{children}</AppLayout>
    </PageToolbarProvider>
  )
}
