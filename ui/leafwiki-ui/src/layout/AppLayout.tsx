import UserToolbar from '@/components/UserToolbar'
import Sidebar from './Sidebar'

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen bg-gray-50 font-sans text-gray-900">
      <Sidebar />
      <div className="flex-1 flex flex-col overflow-auto">
        <header className="flex items-center justify-between p-4 border-b bg-white shadow-sm">
          <UserToolbar />
        </header>
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  )
}
