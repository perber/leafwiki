import AppLayout from '@/layout/AppLayout'

export default function ReadOnlyWrapper({
  children,
}: {
  children: React.ReactNode
}) {
  return <AppLayout>{children}</AppLayout>
}
