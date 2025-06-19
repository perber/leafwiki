import { DialogManger } from '@/components/DialogManager'
import { Button } from '@/components/ui/button'
import { TooltipProvider } from '@/components/ui/tooltip'
import { usePageToolbar } from '@/components/usePageToolbar'
import UserToolbar from '@/components/UserToolbar'
import Sidebar from '@/features/sidebar/Sidebar'
import { useAutoCloseSidebarOnMobile } from '@/lib/useAutoCloseSidebarOnMobile'
import { useSidebarStore } from '@/stores/sidebar'
import { MenuIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { content, titleBar } = usePageToolbar()
  const location = useLocation()
  const [isEditor, setIsEditor] = useState(location.pathname.startsWith('/e/'))

  const sidebarVisible = useSidebarStore((s) => s.sidebarVisible)
  const setSidebarVisible = useSidebarStore((s) => s.setSidebarVisible)

  useAutoCloseSidebarOnMobile()

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      setIsEditor(location.pathname.startsWith('/e/'))
    })
    return () => cancelAnimationFrame(frame)
  }, [location.pathname])

  const mainContainerStyle = !isEditor ? 'overflow-auto p-6' : 'overflow-hidden'

  return (
    <TooltipProvider delayDuration={300}>
      <DialogManger />
      {/* Header */}
      <header className="h-[85px] border-b bg-white p-4 shadow-sm">
        <div className="flex h-full items-center justify-start">
          <div className="flex min-h-full w-6 items-center">
            {/* Sidebar Toggle Button */}
            <Button
              variant={'secondary'}
              className="relative z-20 p-2 text-gray-500 hover:text-gray-800 focus:outline-none"
              onClick={() => setSidebarVisible(!sidebarVisible)}
              aria-label="Toggle Sidebar"
              aria-expanded={sidebarVisible}
            >
              <MenuIcon className="h-5 w-5" />
            </Button>
          </div>
          {/* Left side: Logo and Title */}
          <div className="ml-6 mr-6 flex min-h-full items-center gap-2">
            <h2 className="text-xl font-bold">
              <Link to="/">
                🌿 <span className="max-md:hidden">LeafWiki</span>
              </Link>
            </h2>
          </div>
          <div className="flex min-h-full flex-1 items-center justify-center">
            {titleBar}
          </div>
          <div className="flex min-h-full items-center gap-4">
            {content}
            <UserToolbar />
          </div>
        </div>
      </header>
      <div className="flex h-[calc(100vh-85px)] transition-all duration-200">
        <div
          className={`z-20 h-full overflow-auto border-r border-gray-200 bg-white transition-all duration-200 max-sm:fixed max-sm:h-[calc(100vh-85px)] ${
            sidebarVisible ? 'w-96' : 'w-0'
          }`}
        >
          <Sidebar />
        </div>

        <main
          className={`${mainContainerStyle} flex-1 transition-all duration-200`}
        >
          {children}
        </main>
      </div>
    </TooltipProvider>
  )
}
